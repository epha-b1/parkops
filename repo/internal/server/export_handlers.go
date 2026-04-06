package server

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"parkops/internal/auth"
	"parkops/internal/exports"
)

func (h *analyticsHandler) listExports(c *gin.Context) {
	actor, ok := getCurrentUser(c)
	if !ok {
		abortAPIError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	query := `SELECT id::text, requested_by::text, format, scope, segment_id::text, status, created_at, completed_at FROM exports`
	args := []any{}
	if !auth.HasAnyRole(actor.Roles, []string{auth.RoleFacilityAdmin}) {
		query += ` WHERE requested_by = $1::uuid`
		args = append(args, actor.ID)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := h.pool.Query(c.Request.Context(), query, args...)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	defer rows.Close()

	out := make([]gin.H, 0)
	for rows.Next() {
		var id, format, scope, status string
		var requestedBy, segmentID *string
		var createdAt time.Time
		var completedAt *time.Time
		if err := rows.Scan(&id, &requestedBy, &format, &scope, &segmentID, &status, &createdAt, &completedAt); err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
		if segmentID != nil && *segmentID != "" && !auth.HasAnyRole(actor.Roles, []string{auth.RoleFacilityAdmin}) {
			if allowed, _ := h.segmentAuth.CheckAccess(c.Request.Context(), actor, *segmentID); !allowed {
				continue
			}
		}
		item := gin.H{"id": id, "format": format, "scope": scope, "status": status, "created_at": createdAt.UTC().Format(timeRFC3339)}
		if requestedBy != nil {
			item["requested_by"] = *requestedBy
		}
		if completedAt != nil {
			item["completed_at"] = completedAt.UTC().Format(timeRFC3339)
		}
		out = append(out, item)
	}
	c.JSON(http.StatusOK, out)
}

func (h *analyticsHandler) createExport(c *gin.Context) {
	var b struct {
		Format    string  `json:"format"`
		Scope     string  `json:"scope"`
		SegmentID *string `json:"segment_id"`
		From      *string `json:"from"`
		To        *string `json:"to"`
	}
	if c.ShouldBindJSON(&b) != nil || b.Format == "" || b.Scope == "" {
		abortAPIError(c, 400, "VALIDATION_ERROR", "format and scope required")
		return
	}
	format := exports.Format(b.Format)
	if !format.Valid() {
		abortAPIError(c, 400, "VALIDATION_ERROR", "invalid format or scope")
		return
	}
	validScopes := map[string]bool{"occupancy": true, "bookings": true, "exceptions": true}
	if !validScopes[b.Scope] {
		abortAPIError(c, 400, "VALIDATION_ERROR", "invalid format or scope")
		return
	}

	actor, _ := getCurrentUser(c)

	// Segment authorization
	if b.SegmentID != nil && strings.TrimSpace(*b.SegmentID) != "" {
		allowed, _ := h.segmentAuth.CheckAccess(c.Request.Context(), actor, *b.SegmentID)
		if !allowed {
			abortAPIError(c, 403, "FORBIDDEN", "segment access denied")
			return
		}
	}

	var queryFrom, queryTo any
	if b.From != nil {
		if t, err := time.Parse(time.RFC3339, *b.From); err == nil {
			queryFrom = t.UTC()
		}
	}
	if b.To != nil {
		if t, err := time.Parse(time.RFC3339, *b.To); err == nil {
			queryTo = t.UTC()
		}
	}
	var segmentID any
	if b.SegmentID != nil && strings.TrimSpace(*b.SegmentID) != "" {
		segmentID = *b.SegmentID
	}

	// Insert pending record
	var id string
	err := h.pool.QueryRow(c.Request.Context(), `
		INSERT INTO exports(requested_by, format, scope, segment_id, query_from, query_to, status)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, 'pending')
		RETURNING id::text
	`, actor.ID, b.Format, b.Scope, segmentID, queryFrom, queryTo).Scan(&id)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}

	// Resolve segment members for data-scope filtering — hard fail on error
	// to prevent silent fallback to unfiltered export
	var segmentMemberIDs []string
	if b.SegmentID != nil && strings.TrimSpace(*b.SegmentID) != "" {
		var resolveErr error
		segmentMemberIDs, resolveErr = h.segmentAuth.ResolveMembers(c.Request.Context(), *b.SegmentID)
		if resolveErr != nil {
			h.failExport(c, id)
			abortAPIError(c, 500, "INTERNAL_ERROR", "segment resolution failed")
			return
		}
	}

	// Fetch data, generate file, write to disk
	headers, rows, totalRows, err := h.fetchExportRows(c, b.Scope, 0, segmentMemberIDs)
	if err != nil {
		h.failExport(c, id)
		abortAPIError(c, 500, "INTERNAL_ERROR", "export generation failed")
		return
	}
	truncated := false
	result, err := exports.Generate(format, headers, rows, totalRows, truncated)
	if err != nil {
		h.failExport(c, id)
		abortAPIError(c, 500, "INTERNAL_ERROR", "export generation failed")
		return
	}
	filePath, err := h.fileStore.Write(id, format, result.Data)
	if err != nil {
		h.failExport(c, id)
		abortAPIError(c, 500, "INTERNAL_ERROR", "export storage failed")
		return
	}

	// Mark ready
	_, _ = h.pool.Exec(c.Request.Context(), `UPDATE exports SET status='ready', file_path=$2, completed_at=now() WHERE id=$1::uuid`, id, filePath)
	_ = h.authService.WriteAuditLog(c.Request.Context(), &actor.ID, "export_create", "export", &id, map[string]any{"format": b.Format, "scope": b.Scope, "truncated": result.Truncated})

	c.JSON(http.StatusCreated, gin.H{"id": id, "format": b.Format, "scope": b.Scope, "status": "ready", "truncated": result.Truncated, "created_at": time.Now().UTC().Format(timeRFC3339)})
}

func (h *analyticsHandler) downloadExport(c *gin.Context) {
	actor, ok := getCurrentUser(c)
	if !ok {
		abortAPIError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	var status, formatStr, filePath string
	var requestedBy, segmentID *string
	err := h.pool.QueryRow(c.Request.Context(), `
		SELECT status, format, COALESCE(file_path,''), requested_by::text, segment_id::text FROM exports WHERE id=$1::uuid
	`, c.Param("id")).Scan(&status, &formatStr, &filePath, &requestedBy, &segmentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			abortAPIError(c, 404, "NOT_FOUND", "export not found")
			return
		}
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}

	if !auth.HasAnyRole(actor.Roles, []string{auth.RoleFacilityAdmin}) {
		if requestedBy == nil || *requestedBy != actor.ID {
			abortAPIError(c, http.StatusForbidden, "FORBIDDEN", "forbidden")
			return
		}
		if segmentID != nil && *segmentID != "" {
			if allowed, _ := h.segmentAuth.CheckAccess(c.Request.Context(), actor, *segmentID); !allowed {
				abortAPIError(c, http.StatusForbidden, "FORBIDDEN", "segment access denied")
				return
			}
		}
	}
	if status != "ready" || filePath == "" {
		abortAPIError(c, 404, "NOT_FOUND", "export not ready")
		return
	}

	format := exports.Format(formatStr)
	data, err := h.fileStore.Read(filePath)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "export file not found")
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=export%s", format.Extension()))
	c.Data(http.StatusOK, format.ContentType(), data)
}

func (h *analyticsHandler) failExport(c *gin.Context, id string) {
	_, _ = h.pool.Exec(c.Request.Context(), `UPDATE exports SET status='failed', completed_at=now() WHERE id=$1::uuid`, id)
}

func (h *analyticsHandler) fetchExportRows(c *gin.Context, scope string, limit int, segmentMemberIDs []string) ([]string, [][]string, int, error) {
	ctx := c.Request.Context()
	hasSegment := len(segmentMemberIDs) > 0

	switch scope {
	case "occupancy":
		whereClause := ""
		args := []any{}
		paramIdx := 1
		if hasSegment {
			whereClause = fmt.Sprintf(` WHERE cs.zone_id IN (SELECT DISTINCT r.zone_id FROM reservations r WHERE r.member_id = ANY($%d::uuid[]))`, paramIdx)
			args = append(args, segmentMemberIDs)
			paramIdx++
		}
		var total int
		countQ := `SELECT COUNT(*) FROM capacity_snapshots cs` + whereClause
		if err := h.pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
			return nil, nil, 0, err
		}
		query := `SELECT cs.snapshot_at, cs.zone_id::text, cs.authoritative_stalls FROM capacity_snapshots cs` + whereClause + ` ORDER BY cs.snapshot_at DESC`
		if limit > 0 {
			query += fmt.Sprintf(` LIMIT $%d`, paramIdx)
			args = append(args, limit)
		}
		rows, err := h.pool.Query(ctx, query, args...)
		if err != nil {
			return nil, nil, 0, err
		}
		defer rows.Close()
		out := make([][]string, 0)
		for rows.Next() {
			var at time.Time
			var zoneID string
			var stalls int
			if err := rows.Scan(&at, &zoneID, &stalls); err != nil {
				return nil, nil, 0, err
			}
			out = append(out, []string{at.UTC().Format(timeRFC3339), zoneID, fmt.Sprintf("%d", stalls)})
		}
		return []string{"snapshot_at", "zone_id", "authoritative_stalls"}, out, total, nil

	case "bookings":
		whereClause := ""
		args := []any{}
		paramIdx := 1
		if hasSegment {
			whereClause = fmt.Sprintf(` WHERE member_id = ANY($%d::uuid[])`, paramIdx)
			args = append(args, segmentMemberIDs)
			paramIdx++
		}
		var total int
		countQ := `SELECT COUNT(*) FROM reservations` + whereClause
		if err := h.pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
			return nil, nil, 0, err
		}
		query := `SELECT id::text, zone_id::text, status, stall_count, created_at FROM reservations` + whereClause + ` ORDER BY created_at DESC`
		if limit > 0 {
			query += fmt.Sprintf(` LIMIT $%d`, paramIdx)
			args = append(args, limit)
		}
		rows, err := h.pool.Query(ctx, query, args...)
		if err != nil {
			return nil, nil, 0, err
		}
		defer rows.Close()
		out := make([][]string, 0)
		for rows.Next() {
			var id, zoneID, status string
			var stallCount int
			var createdAt time.Time
			if err := rows.Scan(&id, &zoneID, &status, &stallCount, &createdAt); err != nil {
				return nil, nil, 0, err
			}
			out = append(out, []string{id, zoneID, status, fmt.Sprintf("%d", stallCount), createdAt.UTC().Format(timeRFC3339)})
		}
		return []string{"id", "zone_id", "status", "stall_count", "created_at"}, out, total, nil

	case "exceptions":
		whereClause := ""
		args := []any{}
		paramIdx := 1
		if hasSegment {
			whereClause = fmt.Sprintf(` WHERE e.device_id IN (SELECT d.id FROM devices d WHERE d.organization_id IN (SELECT DISTINCT m.organization_id FROM members m WHERE m.id = ANY($%d::uuid[])))`, paramIdx)
			args = append(args, segmentMemberIDs)
			paramIdx++
		}
		var total int
		countQ := `SELECT COUNT(*) FROM exceptions e` + whereClause
		if err := h.pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
			return nil, nil, 0, err
		}
		query := `SELECT e.id::text, COALESCE(e.exception_type,'unknown'), e.status, e.created_at FROM exceptions e` + whereClause + ` ORDER BY e.created_at DESC`
		if limit > 0 {
			query += fmt.Sprintf(` LIMIT $%d`, paramIdx)
			args = append(args, limit)
		}
		rows, err := h.pool.Query(ctx, query, args...)
		if err != nil {
			return nil, nil, 0, err
		}
		defer rows.Close()
		out := make([][]string, 0)
		for rows.Next() {
			var id, etype, status string
			var createdAt time.Time
			if err := rows.Scan(&id, &etype, &status, &createdAt); err != nil {
				return nil, nil, 0, err
			}
			out = append(out, []string{id, etype, status, createdAt.UTC().Format(timeRFC3339)})
		}
		return []string{"id", "exception_type", "status", "created_at"}, out, total, nil
	}
	return nil, nil, 0, fmt.Errorf("invalid scope")
}
