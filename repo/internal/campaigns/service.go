package campaigns

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"parkops/internal/notifications"
)

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func (s *Service) ProcessDueTaskReminders(ctx context.Context, now time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var topicID string
	err = tx.QueryRow(ctx, `SELECT id::text FROM notification_topics WHERE name='task_reminder' LIMIT 1`).Scan(&topicID)
	if err != nil {
		return err
	}

	tasksRows, err := tx.Query(ctx, `
		SELECT t.id::text, t.description, COALESCE(c.target_role, '')
		FROM tasks t
		JOIN campaigns c ON c.id = t.campaign_id
		WHERE t.completed_at IS NULL
		  AND t.deadline IS NOT NULL
		  AND t.deadline <= $1
		  AND (
			  t.last_reminder_at IS NULL
			  OR t.last_reminder_at <= $1 - ((t.reminder_interval_minutes::text || ' minutes')::interval)
		  )
		ORDER BY t.deadline ASC
		LIMIT 200
	`, now.UTC())
	if err != nil {
		return err
	}
	defer tasksRows.Close()

	type dueTask struct {
		ID          string
		Description string
		TargetRole  string
	}
	tasks := make([]dueTask, 0)
	for tasksRows.Next() {
		var t dueTask
		if err := tasksRows.Scan(&t.ID, &t.Description, &t.TargetRole); err != nil {
			return err
		}
		tasks = append(tasks, t)
	}

	for _, task := range tasks {
		// Resolve recipients: subscribers filtered by campaign target_role
		var usersRows pgx.Rows
		if task.TargetRole != "" {
			usersRows, err = tx.Query(ctx, `
				SELECT DISTINCT ns.user_id::text
				FROM notification_subscriptions ns
				JOIN user_roles ur ON ur.user_id = ns.user_id
				JOIN roles r ON r.id = ur.role_id
				WHERE ns.topic_id = $1::uuid AND r.name = $2
			`, topicID, task.TargetRole)
		} else {
			usersRows, err = tx.Query(ctx, `SELECT DISTINCT user_id::text FROM notification_subscriptions WHERE topic_id=$1::uuid`, topicID)
		}
		if err != nil {
			return err
		}
		userIDs := make([]string, 0)
		for usersRows.Next() {
			var userID string
			if err := usersRows.Scan(&userID); err != nil {
				usersRows.Close()
				return err
			}
			userIDs = append(userIDs, userID)
		}
		usersRows.Close()

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

			var notificationID string
			err = tx.QueryRow(ctx, `
				INSERT INTO notifications(user_id, topic_id, title, body, created_at)
				VALUES ($1::uuid, $2::uuid, 'Task reminder', $3, $4)
				RETURNING id::text
			`, userID, topicID, task.Description, now.UTC()).Scan(&notificationID)
			if err != nil {
				return err
			}

			payload, _ := json.Marshal(map[string]any{"recipient": userID, "task_id": task.ID})
			status := "pending"
			nextAttemptAt := any(nil)
			if dndEnabled && notifications.InDNDWindow(now.UTC(), startT, endT) {
				status = "deferred"
				nextAttemptAt = notifications.DNDEnd(now.UTC(), startT, endT)
			}
			_, err = tx.Exec(ctx, `
				INSERT INTO notification_jobs(notification_id, user_id, topic_id, channel, status, attempt_count, next_attempt_at, payload, created_at, updated_at)
				VALUES ($1::uuid, $2::uuid, $3::uuid, 'in_app', $4, 0, $5, $6::jsonb, $7, $7)
			`, notificationID, userID, topicID, status, nextAttemptAt, string(payload), now.UTC())
			if err != nil {
				return err
			}
		}

		_, err = tx.Exec(ctx, `UPDATE tasks SET last_reminder_at=$2, updated_at=$2 WHERE id=$1::uuid`, task.ID, now.UTC())
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func StartReminderScheduler(ctx context.Context, logger *slog.Logger, service *Service) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if err := service.ProcessDueTaskReminders(ctx, now.UTC()); err != nil && err != pgx.ErrNoRows {
				logger.Error("task reminder processor failed", "error", err)
			}
		}
	}
}
