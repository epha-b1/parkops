package notifications

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func (s *Service) QueueBookingConfirmed(ctx context.Context, tx pgx.Tx, bookingID string, at time.Time) error {
	return s.queueByTrigger(ctx, tx, "booking.confirmed", "booking_success", "Booking confirmed", "Your booking has been confirmed.", bookingID, at)
}

func (s *Service) queueByTrigger(ctx context.Context, tx pgx.Tx, triggerEvent, fallbackTopic, fallbackTitle, fallbackBody, bookingID string, at time.Time) error {
	type dispatchRule struct {
		TopicID  string
		Title    string
		Body     string
	}

	rules := make([]dispatchRule, 0)
	ruleRows, err := tx.Query(ctx, `
		SELECT mr.topic_id::text, COALESCE(NULLIF(mr.template,''), $2)
		FROM message_rules mr
		WHERE mr.active = true AND mr.trigger_event = $1
		ORDER BY mr.created_at ASC
	`, triggerEvent, fallbackBody)
	if err != nil {
		return err
	}
	for ruleRows.Next() {
		var topicID, template string
		if err := ruleRows.Scan(&topicID, &template); err != nil {
			ruleRows.Close()
			return err
		}
		rules = append(rules, dispatchRule{TopicID: topicID, Title: fallbackTitle, Body: template})
	}
	ruleRows.Close()

	if len(rules) == 0 {
		var topicID string
		err := tx.QueryRow(ctx, `SELECT id::text FROM notification_topics WHERE name=$1 LIMIT 1`, fallbackTopic).Scan(&topicID)
		if err != nil {
			return err
		}
		rules = append(rules, dispatchRule{TopicID: topicID, Title: fallbackTitle, Body: fallbackBody})
	}

	for _, rule := range rules {
		rows, err := tx.Query(ctx, `
			SELECT ns.user_id::text
			FROM notification_subscriptions ns
			WHERE ns.topic_id = $1::uuid
		`, rule.TopicID)
		if err != nil {
			return err
		}

		userIDs := make([]string, 0)
		for rows.Next() {
			var userID string
			if err := rows.Scan(&userID); err != nil {
				rows.Close()
				return err
			}
			userIDs = append(userIDs, userID)
		}
		rows.Close()

		for _, userID := range userIDs {
			var startT, endT time.Time
			var dndEnabled bool
			err := tx.QueryRow(ctx, `
				SELECT start_time::time, end_time::time, enabled
				FROM user_dnd_settings
				WHERE user_id = $1::uuid
			`, userID).Scan(&startT, &endT, &dndEnabled)
			if err == pgx.ErrNoRows {
				dndEnabled = false
			} else if err != nil {
				return err
			}

			var existingToday int
			err = tx.QueryRow(ctx, `
				SELECT COUNT(*)
				FROM notification_jobs
				WHERE user_id=$1::uuid
				  AND booking_id=$2::uuid
				  AND created_at >= date_trunc('day', $3::timestamptz)
				  AND created_at < date_trunc('day', $3::timestamptz) + interval '1 day'
			`, userID, bookingID, at.UTC()).Scan(&existingToday)
			if err != nil {
				return err
			}
			if !AllowByFrequencyCap(existingToday) {
				continue
			}

			var notificationID string
			err = tx.QueryRow(ctx, `
				INSERT INTO notifications(user_id, topic_id, title, body, booking_id, created_at)
				VALUES ($1::uuid, $2::uuid, $3, $4, $5::uuid, $6)
				RETURNING id::text
			`, userID, rule.TopicID, rule.Title, rule.Body, bookingID, at.UTC()).Scan(&notificationID)
			if err != nil {
				return err
			}

			status := "delivered"
			nextAttemptAt := any(nil)
			deliveredAt := any(at.UTC())
			if dndEnabled && InDNDWindow(at.UTC(), startT, endT) {
				status = "deferred"
				nextAttemptAt = DNDEnd(at.UTC(), startT, endT)
				deliveredAt = nil
			}

			payload, _ := json.Marshal(map[string]any{"recipient": userID, "booking_id": bookingID, "trigger_event": triggerEvent})
			_, err = tx.Exec(ctx, `
				INSERT INTO notification_jobs(notification_id, user_id, booking_id, topic_id, channel, status, attempt_count, next_attempt_at, payload, created_at, updated_at, delivered_at)
				VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, 'in_app', $5, 0, $6, $7::jsonb, $8, $8, $9)
			`, notificationID, userID, bookingID, rule.TopicID, status, nextAttemptAt, string(payload), at.UTC(), deliveredAt)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *Service) ProcessDueJobs(ctx context.Context, now time.Time) error {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, attempt_count
		FROM notification_jobs
		WHERE status IN ('pending','failed','deferred')
		  AND (next_attempt_at IS NULL OR next_attempt_at <= $1)
		ORDER BY created_at ASC
		LIMIT 200
	`, now.UTC())
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		var attempts int
		if err := rows.Scan(&id, &attempts); err != nil {
			return err
		}
		_, err = s.pool.Exec(ctx, `
			UPDATE notification_jobs
			SET status='delivered', delivered_at=$2, updated_at=$2
			WHERE id=$1::uuid
		`, id, now.UTC())
		if err != nil {
			if d, ok := RetryBackoff(attempts); ok {
				_, _ = s.pool.Exec(ctx, `
					UPDATE notification_jobs
					SET status='failed', attempt_count=attempt_count+1, next_attempt_at=$2, last_error='delivery failed', updated_at=$3
					WHERE id=$1::uuid
				`, id, now.UTC().Add(d), now.UTC())
			} else {
				_, _ = s.pool.Exec(ctx, `
					UPDATE notification_jobs
					SET status='suppressed', last_error='max retries exceeded', updated_at=$2
					WHERE id=$1::uuid
				`, id, now.UTC())
			}
		}
	}
	return nil
}

func StartProcessor(ctx context.Context, logger *slog.Logger, service *Service) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if err := service.ProcessDueJobs(ctx, now.UTC()); err != nil {
				logger.Error("notification processor failed", "error", err)
			}
		}
	}
}
