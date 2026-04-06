package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"parkops/internal/auth"
	"parkops/internal/exports"
)

type analyticsHandler struct {
	pool        *pgxpool.Pool
	authService *auth.Service
	segmentAuth *exports.SegmentAuthorizer
	fileStore   *exports.FileStore
}

func registerAnalyticsRoutes(r *gin.Engine, authService *auth.Service, pool *pgxpool.Pool, segmentAuth *exports.SegmentAuthorizer, fileStore *exports.FileStore) {
	h := &analyticsHandler{pool: pool, authService: authService, segmentAuth: segmentAuth, fileStore: fileStore}

	read := r.Group("/api")
	read.Use(requireSession(authService), enforceForcePasswordChange(), requireRoles(authService, allSystemRoles()...))
	{
		read.GET("/analytics/occupancy", h.occupancy)
		read.GET("/analytics/bookings", h.bookings)
		read.GET("/analytics/exceptions", h.exceptions)
		read.GET("/exports", h.listExports)
		read.GET("/exports/:id/download", h.downloadExport)
	}

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
	case "revenue":
		groupCol = "rp.name"
		labelExpr = "COALESCE(rp.name, 'No Plan')"
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
		LEFT JOIN rate_plans rp ON rp.id = r.rate_plan_id
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
