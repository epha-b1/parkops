package server

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"parkops/internal/auth"
)

type analyticsHandler struct {
	pool        *pgxpool.Pool
	authService *auth.Service
}

func registerAnalyticsRoutes(r *gin.Engine, authService *auth.Service, pool *pgxpool.Pool) {
	h := &analyticsHandler{pool: pool, authService: authService}

	// Analytics read — all authenticated roles
	read := r.Group("/api")
	read.Use(requireSession(authService), enforceForcePasswordChange(), requireRoles(authService, allSystemRoles()...))
	{
		read.GET("/analytics/occupancy", h.occupancy)
		read.GET("/analytics/bookings", h.bookings)
		read.GET("/analytics/exceptions", h.exceptions)
		read.GET("/exports", h.listExports)
		read.GET("/exports/:id/download", h.downloadExport)
	}

	// Export creation — admin and dispatch only
	write := r.Group("/api")
	write.Use(requireSession(authService), enforceForcePasswordChange(), requireRoles(authService, auth.RoleFacilityAdmin, auth.RoleDispatch))
	{
		write.POST("/exports", h.createExport)
	}
}

func (h *analyticsHandler) occupancy(c *gin.Context) {
	from := c.Query("from")
	to := c.Query("to")
	if from == "" || to == "" {
		abortAPIError(c, 400, "VALIDATION_ERROR", "from and to query params required")
		return
	}
	fromT, err1 := time.Parse(time.RFC3339, from)
	toT, err2 := time.Parse(time.RFC3339, to)
	if err1 != nil || err2 != nil {
		abortAPIError(c, 400, "VALIDATION_ERROR", "from and to must be RFC3339")
		return
	}

	granularity := c.DefaultQuery("granularity", "day")
	var truncFn string
	switch granularity {
	case "hour":
		truncFn = "hour"
	case "week":
		truncFn = "week"
	default:
		truncFn = "day"
	}

	zoneFilter := ""
	args := []any{fromT.UTC(), toT.UTC()}
	if zoneID := c.Query("zone_id"); zoneID != "" {
		zoneFilter = " AND cs.zone_id = $3::uuid"
		args = append(args, zoneID)
	}

	query := fmt.Sprintf(`
		SELECT date_trunc('%s', cs.snapshot_at) AS period,
			ROUND(AVG(CASE WHEN z.total_stalls > 0 THEN (z.total_stalls - cs.authoritative_stalls)::numeric / z.total_stalls * 100 ELSE 0 END), 2) AS avg_occ,
			ROUND(MAX(CASE WHEN z.total_stalls > 0 THEN (z.total_stalls - cs.authoritative_stalls)::numeric / z.total_stalls * 100 ELSE 0 END), 2) AS peak_occ
		FROM capacity_snapshots cs
		JOIN zones z ON z.id = cs.zone_id
		WHERE cs.snapshot_at >= $1 AND cs.snapshot_at <= $2%s
		GROUP BY period ORDER BY period
	`, truncFn, zoneFilter)

	rows, err := h.pool.Query(c.Request.Context(), query, args...)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	defer rows.Close()
	out := make([]gin.H, 0)
	for rows.Next() {
		var period time.Time
		var avgOcc, peakOcc float64
		if err := rows.Scan(&period, &avgOcc, &peakOcc); err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
		out = append(out, gin.H{"period": period.UTC().Format(timeRFC3339), "avg_occupancy_pct": avgOcc, "peak_occupancy_pct": peakOcc})
	}
	c.JSON(http.StatusOK, out)
}

func (h *analyticsHandler) bookings(c *gin.Context) {
	pivotBy := c.DefaultQuery("pivot_by", "time")

	var groupCol, labelExpr string
	switch pivotBy {
	case "region":
		groupCol = "f.name"
		labelExpr = "COALESCE(f.name, 'Unknown')"
	case "category":
		groupCol = "z.name"
		labelExpr = "COALESCE(z.name, 'Unknown')"
	default:
		groupCol = "date_trunc('day', r.created_at)"
		labelExpr = "date_trunc('day', r.created_at)::text"
	}

	args := []any{}
	whereClause := "WHERE 1=1"
	paramIdx := 1
	if from := c.Query("from"); from != "" {
		whereClause += fmt.Sprintf(" AND r.created_at >= $%d", paramIdx)
		t, _ := time.Parse(time.RFC3339, from)
		args = append(args, t.UTC())
		paramIdx++
	}
	if to := c.Query("to"); to != "" {
		whereClause += fmt.Sprintf(" AND r.created_at <= $%d", paramIdx)
		t, _ := time.Parse(time.RFC3339, to)
		args = append(args, t.UTC())
		paramIdx++
	}

	query := fmt.Sprintf(`
		SELECT %s AS label, COUNT(*) AS cnt, COALESCE(SUM(r.stall_count),0) AS total_stalls
		FROM reservations r
		LEFT JOIN zones z ON z.id = r.zone_id
		LEFT JOIN lots l ON l.id = z.lot_id
		LEFT JOIN facilities f ON f.id = l.facility_id
		%s
		GROUP BY %s ORDER BY %s
	`, labelExpr, whereClause, groupCol, groupCol)

	rows, err := h.pool.Query(c.Request.Context(), query, args...)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	defer rows.Close()
	out := make([]gin.H, 0)
	for rows.Next() {
		var label string
		var count int
		var totalStalls int64
		if err := rows.Scan(&label, &count, &totalStalls); err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
		out = append(out, gin.H{"label": label, "count": count, "total_stalls": totalStalls})
	}
	c.JSON(http.StatusOK, out)
}

func (h *analyticsHandler) exceptions(c *gin.Context) {
	args := []any{}
	whereClause := "WHERE 1=1"
	paramIdx := 1
	if from := c.Query("from"); from != "" {
		whereClause += fmt.Sprintf(" AND e.created_at >= $%d", paramIdx)
		t, _ := time.Parse(time.RFC3339, from)
		args = append(args, t.UTC())
		paramIdx++
	}
	if to := c.Query("to"); to != "" {
		whereClause += fmt.Sprintf(" AND e.created_at <= $%d", paramIdx)
		t, _ := time.Parse(time.RFC3339, to)
		args = append(args, t.UTC())
		paramIdx++
	}

	query := fmt.Sprintf(`
		SELECT COALESCE(e.exception_type, 'unknown') AS etype, COUNT(*) AS cnt
		FROM exceptions e %s
		GROUP BY etype ORDER BY cnt DESC
	`, whereClause)

	rows, err := h.pool.Query(c.Request.Context(), query, args...)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	defer rows.Close()
	out := make([]gin.H, 0)
	for rows.Next() {
		var etype string
		var count int
		if err := rows.Scan(&etype, &count); err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
		out = append(out, gin.H{"exception_type": etype, "count": count})
	}
	c.JSON(http.StatusOK, out)
}

func (h *analyticsHandler) listExports(c *gin.Context) {
	actor, ok := getCurrentUser(c)
	if !ok {
		abortAPIError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	query := `
		SELECT id::text, requested_by::text, format, scope, segment_id::text, status, created_at, completed_at
		FROM exports
	`
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
		// Segment-scoped exports: non-admin must pass segment membership check
		if segmentID != nil && *segmentID != "" && !auth.HasAnyRole(actor.Roles, []string{auth.RoleFacilityAdmin}) {
			if !h.isActorInSegmentScope(c, actor, *segmentID) {
				continue // skip exports the actor has no segment access to
			}
		}
		item := gin.H{
			"id": id, "format": format, "scope": scope, "status": status,
			"created_at": createdAt.UTC().Format(timeRFC3339),
		}
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
	validFormats := map[string]bool{"csv": true, "excel": true, "pdf": true}
	validScopes := map[string]bool{"occupancy": true, "bookings": true, "exceptions": true}
	if !validFormats[b.Format] || !validScopes[b.Scope] {
		abortAPIError(c, 400, "VALIDATION_ERROR", "invalid format or scope")
		return
	}

	actor, _ := getCurrentUser(c)

	// Segment access check: role AND segment-scope membership
	if b.SegmentID != nil && strings.TrimSpace(*b.SegmentID) != "" {
		if !h.checkSegmentAccess(c, actor, *b.SegmentID) {
			return
		}
	}

	var queryFrom, queryTo any
	if b.From != nil {
		t, err := time.Parse(time.RFC3339, *b.From)
		if err == nil {
			queryFrom = t.UTC()
		}
	}
	if b.To != nil {
		t, err := time.Parse(time.RFC3339, *b.To)
		if err == nil {
			queryTo = t.UTC()
		}
	}

	var segmentID any
	if b.SegmentID != nil && strings.TrimSpace(*b.SegmentID) != "" {
		segmentID = *b.SegmentID
	}

	// Create export record and generate content inline.
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

	content, truncated, err := h.generateExportContent(c, b.Format, b.Scope, queryFrom, queryTo)
	if err != nil {
		_, _ = h.pool.Exec(c.Request.Context(), `UPDATE exports SET status='failed', completed_at=now() WHERE id=$1::uuid`, id)
		abortAPIError(c, 500, "INTERNAL_ERROR", "export generation failed")
		return
	}

	_, err = h.pool.Exec(c.Request.Context(), `
		UPDATE exports SET status='ready', file_path=$2, completed_at=now() WHERE id=$1::uuid
	`, id, content)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}

	_ = h.authService.WriteAuditLog(c.Request.Context(), &actor.ID, "export_create", "export", &id, map[string]any{"format": b.Format, "scope": b.Scope, "truncated": truncated})

	c.JSON(http.StatusCreated, gin.H{"id": id, "format": b.Format, "scope": b.Scope, "status": "ready", "truncated": truncated, "created_at": time.Now().UTC().Format(timeRFC3339)})
}

func (h *analyticsHandler) generateExportContent(c *gin.Context, format, scope string, from, to any) (string, bool, error) {
	limit := 0

	headers, rows, totalRows, err := h.fetchExportRows(c, scope, limit)
	if err != nil {
		return "", false, err
	}
	truncated := limit > 0 && totalRows > limit

	switch format {
	case "csv":
		return generateCSV(headers, rows, totalRows, truncated, limit)
	case "excel":
		return generateExcel(headers, rows, totalRows, truncated, limit)
	case "pdf":
		return generatePDF(headers, rows, totalRows, truncated, limit)
	default:
		return "", false, fmt.Errorf("unsupported format")
	}
}

func generateCSV(headers []string, rows [][]string, totalRows int, truncated bool, limit int) (string, bool, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if truncated {
		_ = w.Write([]string{fmt.Sprintf("NOTE: truncated export from %d to %d rows", totalRows, len(rows))})
	}
	_ = w.Write(headers)
	for _, row := range rows {
		_ = w.Write(row)
	}
	w.Flush()
	return buf.String(), truncated, nil
}

func generateExcel(headers []string, rows [][]string, totalRows int, truncated bool, limit int) (string, bool, error) {
	var buf bytes.Buffer
	if truncated {
		buf.WriteString(fmt.Sprintf("NOTE: truncated export from %d to %d rows\n", totalRows, len(rows)))
	}
	buf.WriteString(strings.Join(headers, "\t") + "\n")
	for _, row := range rows {
		buf.WriteString(strings.Join(row, "\t") + "\n")
	}
	return buf.String(), truncated, nil
}

func generatePDF(headers []string, rows [][]string, totalRows int, truncated bool, limit int) (string, bool, error) {
	var buf bytes.Buffer
	buf.WriteString("EXPORT REPORT\n")
	buf.WriteString(strings.Repeat("=", 60) + "\n")
	if truncated {
		buf.WriteString(fmt.Sprintf("NOTE: truncated export from %d to %d rows\n", totalRows, len(rows)))
	}
	buf.WriteString(strings.Join(headers, " | ") + "\n")
	buf.WriteString(strings.Repeat("-", 60) + "\n")
	for _, row := range rows {
		buf.WriteString(strings.Join(row, " | ") + "\n")
	}
	buf.WriteString(strings.Repeat("=", 60) + "\n")
	buf.WriteString(fmt.Sprintf("Total rows: %d\n", len(rows)))
	return buf.String(), truncated, nil
}

func (h *analyticsHandler) fetchExportRows(c *gin.Context, scope string, limit int) ([]string, [][]string, int, error) {
	var buf bytes.Buffer
	_ = buf

	ctx := c.Request.Context()
	switch scope {
	case "occupancy":
		var total int
		if err := h.pool.QueryRow(ctx, `SELECT COUNT(*) FROM capacity_snapshots`).Scan(&total); err != nil {
			return nil, nil, 0, err
		}
		query := `SELECT cs.snapshot_at, cs.zone_id::text, cs.authoritative_stalls FROM capacity_snapshots cs ORDER BY cs.snapshot_at DESC`
		args := []any{}
		if limit > 0 {
			query += ` LIMIT $1`
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
		var total int
		if err := h.pool.QueryRow(ctx, `SELECT COUNT(*) FROM reservations`).Scan(&total); err != nil {
			return nil, nil, 0, err
		}
		query := `SELECT id::text, zone_id::text, status, stall_count, created_at FROM reservations ORDER BY created_at DESC`
		args := []any{}
		if limit > 0 {
			query += ` LIMIT $1`
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
		var total int
		if err := h.pool.QueryRow(ctx, `SELECT COUNT(*) FROM exceptions`).Scan(&total); err != nil {
			return nil, nil, 0, err
		}
		query := `SELECT id::text, COALESCE(exception_type,'unknown'), status, created_at FROM exceptions ORDER BY created_at DESC`
		args := []any{}
		if limit > 0 {
			query += ` LIMIT $1`
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

func (h *analyticsHandler) downloadExport(c *gin.Context) {
	actor, ok := getCurrentUser(c)
	if !ok {
		abortAPIError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	var status, format, filePath string
	var requestedBy, segmentID *string
	err := h.pool.QueryRow(c.Request.Context(), `
		SELECT status, format, COALESCE(file_path,''), requested_by::text, segment_id::text FROM exports WHERE id=$1::uuid
	`, c.Param("id")).Scan(&status, &format, &filePath, &requestedBy, &segmentID)
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
		// Segment-scoped export: verify membership
		if segmentID != nil && *segmentID != "" {
			if !h.isActorInSegmentScope(c, actor, *segmentID) {
				abortAPIError(c, http.StatusForbidden, "FORBIDDEN", "segment access denied")
				return
			}
		}
	}
	if status != "ready" || filePath == "" {
		abortAPIError(c, 404, "NOT_FOUND", "export not ready")
		return
	}

	var contentType, filename string
	switch format {
	case "csv":
		contentType = "text/csv"
		filename = "export.csv"
	case "excel":
		contentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		filename = "export.xlsx"
	case "pdf":
		contentType = "application/pdf"
		filename = "export.pdf"
	default:
		abortAPIError(c, 400, "VALIDATION_ERROR", "format not supported")
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Data(http.StatusOK, contentType, []byte(filePath))
}

// checkSegmentAccess enforces role AND segment-scope membership for exports.
// Returns true if access is allowed; aborts with 403 and returns false otherwise.
func (h *analyticsHandler) checkSegmentAccess(c *gin.Context, actor auth.User, segmentID string) bool {
	// Admins bypass segment membership check
	if auth.HasAnyRole(actor.Roles, []string{auth.RoleFacilityAdmin}) {
		// Still verify the segment exists
		var exists bool
		err := h.pool.QueryRow(c.Request.Context(), `SELECT EXISTS(SELECT 1 FROM segment_definitions WHERE id=$1::uuid)`, segmentID).Scan(&exists)
		if err != nil || !exists {
			abortAPIError(c, 403, "FORBIDDEN", "segment access denied")
			return false
		}
		return true
	}
	// Non-admin: role check (must be dispatch to create exports - already enforced by route)
	// AND segment membership check
	if !h.isActorInSegmentScope(c, actor, segmentID) {
		abortAPIError(c, 403, "FORBIDDEN", "segment access denied")
		return false
	}
	return true
}

// isActorInSegmentScope checks whether the actor's organization is represented
// in the segment's member set (i.e., the segment contains members from the actor's org).
func (h *analyticsHandler) isActorInSegmentScope(c *gin.Context, actor auth.User, segmentID string) bool {
	if actor.OrganizationID == nil {
		return false
	}
	var inScope bool
	err := h.pool.QueryRow(c.Request.Context(), `
		SELECT EXISTS(
			SELECT 1 FROM segment_definitions sd
			WHERE sd.id = $1::uuid
			AND EXISTS(
				SELECT 1 FROM members m
				WHERE m.organization_id = $2::uuid
			)
		)
	`, segmentID, *actor.OrganizationID).Scan(&inScope)
	if err != nil {
		return false
	}
	return inScope
}
