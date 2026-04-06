package segments

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
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

// EvaluateSegment counts members matching the segment's filter_expression.
// Filter expressions support:
//
//	{"tag": "tag_name"}                           — member has tag
//	{"arrears_balance_cents": {"gt": 5000}}       — balance comparison (gt, gte, lt, lte, eq)
//	{"and": [<filter>, ...]}                      — all must match
//	{"or": [<filter>, ...]}                       — any must match
func (s *Service) EvaluateSegment(ctx context.Context, filterExpr json.RawMessage) ([]string, error) {
	where, args, err := buildFilter(filterExpr, 1)
	if err != nil {
		return nil, fmt.Errorf("invalid filter: %w", err)
	}

	query := `SELECT m.id::text FROM members m ` + where
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func buildFilter(raw json.RawMessage, paramIdx int) (string, []any, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "WHERE 1=1", nil, nil
	}

	if tagVal, ok := obj["tag"]; ok {
		var tagName string
		if err := json.Unmarshal(tagVal, &tagName); err != nil {
			return "", nil, fmt.Errorf("tag must be a string")
		}
		where := fmt.Sprintf("JOIN member_tags mt ON mt.member_id = m.id JOIN tags t ON t.id = mt.tag_id WHERE t.name = $%d", paramIdx)
		return where, []any{tagName}, nil
	}

	if balVal, ok := obj["arrears_balance_cents"]; ok {
		var cmp map[string]int64
		if err := json.Unmarshal(balVal, &cmp); err != nil {
			return "", nil, fmt.Errorf("arrears_balance_cents must be {op: value}")
		}
		clauses := []string{}
		args := []any{}
		for op, val := range cmp {
			sqlOp, err := mapOperator(op)
			if err != nil {
				return "", nil, err
			}
			clauses = append(clauses, fmt.Sprintf("m.arrears_balance_cents %s $%d", sqlOp, paramIdx))
			args = append(args, val)
			paramIdx++
		}
		return "WHERE " + strings.Join(clauses, " AND "), args, nil
	}

	if andVal, ok := obj["and"]; ok {
		var filters []json.RawMessage
		if err := json.Unmarshal(andVal, &filters); err != nil {
			return "", nil, fmt.Errorf("and must be an array")
		}
		return buildCompound(filters, "AND", paramIdx)
	}

	if orVal, ok := obj["or"]; ok {
		var filters []json.RawMessage
		if err := json.Unmarshal(orVal, &filters); err != nil {
			return "", nil, fmt.Errorf("or must be an array")
		}
		return buildCompound(filters, "OR", paramIdx)
	}

	return "WHERE 1=1", nil, nil
}

func buildCompound(filters []json.RawMessage, combiner string, paramIdx int) (string, []any, error) {
	if len(filters) == 0 {
		return "WHERE 1=1", nil, nil
	}

	var conditions []string
	var allArgs []any
	needsTagJoin := false

	for _, f := range filters {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(f, &obj); err != nil {
			continue
		}

		if tagVal, ok := obj["tag"]; ok {
			var tagName string
			if err := json.Unmarshal(tagVal, &tagName); err != nil {
				return "", nil, fmt.Errorf("tag must be a string")
			}
			needsTagJoin = true
			conditions = append(conditions, fmt.Sprintf("t.name = $%d", paramIdx))
			allArgs = append(allArgs, tagName)
			paramIdx++
		} else if balVal, ok := obj["arrears_balance_cents"]; ok {
			var cmp map[string]int64
			if err := json.Unmarshal(balVal, &cmp); err != nil {
				return "", nil, fmt.Errorf("arrears_balance_cents must be {op: value}")
			}
			for op, val := range cmp {
				sqlOp, err := mapOperator(op)
				if err != nil {
					return "", nil, err
				}
				conditions = append(conditions, fmt.Sprintf("m.arrears_balance_cents %s $%d", sqlOp, paramIdx))
				allArgs = append(allArgs, val)
				paramIdx++
			}
		}
	}

	if len(conditions) == 0 {
		return "WHERE 1=1", nil, nil
	}

	join := ""
	if needsTagJoin {
		join = "JOIN member_tags mt ON mt.member_id = m.id JOIN tags t ON t.id = mt.tag_id "
	}

	return join + "WHERE (" + strings.Join(conditions, " "+combiner+" ") + ")", allArgs, nil
}

func mapOperator(op string) (string, error) {
	switch op {
	case "gt":
		return ">", nil
	case "gte":
		return ">=", nil
	case "lt":
		return "<", nil
	case "lte":
		return "<=", nil
	case "eq":
		return "=", nil
	default:
		return "", fmt.Errorf("unknown operator: %s", op)
	}
}

// RunSegment evaluates the segment and records a segment_run.
func (s *Service) RunSegment(ctx context.Context, segmentID, triggeredBy string) (string, int, error) {
	var filterExpr json.RawMessage
	err := s.pool.QueryRow(ctx, `SELECT filter_expression FROM segment_definitions WHERE id=$1::uuid`, segmentID).Scan(&filterExpr)
	if err != nil {
		return "", 0, err
	}

	memberIDs, err := s.EvaluateSegment(ctx, filterExpr)
	if err != nil {
		return "", 0, err
	}

	var runID string
	err = s.pool.QueryRow(ctx, `
		INSERT INTO segment_runs(segment_id, member_count, triggered_by)
		VALUES ($1::uuid, $2, $3)
		RETURNING id::text
	`, segmentID, len(memberIDs), triggeredBy).Scan(&runID)
	if err != nil {
		return "", 0, err
	}

	return runID, len(memberIDs), nil
}

// RunNightlySegments evaluates all segments with schedule='nightly'.
func (s *Service) RunNightlySegments(ctx context.Context) error {
	rows, err := s.pool.Query(ctx, `SELECT id::text FROM segment_definitions WHERE schedule='nightly'`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}

	for _, id := range ids {
		if _, _, err := s.RunSegment(ctx, id, "scheduler"); err != nil {
			return err
		}
	}
	return nil
}

// NightlyConfig holds the configurable schedule for nightly segment runs.
type NightlyConfig struct {
	Hour     int
	Minute   int
	Timezone *time.Location
}

// StartNightlyScheduler runs nightly segments at the configured time.
func StartNightlyScheduler(ctx context.Context, logger *slog.Logger, service *Service, cfg NightlyConfig) {
	if cfg.Timezone == nil {
		cfg.Timezone = time.UTC
	}
	logger.Info("nightly segment scheduler configured", "hour", cfg.Hour, "minute", cfg.Minute, "timezone", cfg.Timezone.String())
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			localNow := now.In(cfg.Timezone)
			if localNow.Hour() == cfg.Hour && localNow.Minute() == cfg.Minute {
				if err := service.RunNightlySegments(ctx); err != nil && err != pgx.ErrNoRows {
					logger.Error("nightly segment scheduler failed", "error", err)
				}
			}
		}
	}
}
