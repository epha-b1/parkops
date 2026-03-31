package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"parkops/internal/auth"
)

type reservationHandler struct {
	pool        *pgxpool.Pool
	authService *auth.Service
}

func registerReservationRoutes(r *gin.Engine, authService *auth.Service, pool *pgxpool.Pool) {
	h := &reservationHandler{pool: pool, authService: authService}

	allRead := []string{auth.RoleFacilityAdmin, auth.RoleFleetManager, auth.RoleDispatch, auth.RoleAuditor}
	read := r.Group("/api")
	read.Use(requireSession(authService), enforceForcePasswordChange(), requireRoles(authService, allRead...))
	{
		read.GET("/availability", h.getAvailability)
		read.GET("/reservations", h.listReservations)
		read.GET("/capacity/dashboard", h.capacityDashboard)
		read.GET("/capacity/zones/:id/stalls", h.zoneStalls)
		read.GET("/capacity/snapshots", h.listSnapshots)
		read.GET("/reservations/stats/today", h.reservationStatsToday)
		read.GET("/reservations/:id/timeline", h.reservationTimeline)
		read.GET("/exceptions", h.listOpenExceptions)
	}

	write := r.Group("/api")
	write.Use(requireSession(authService), enforceForcePasswordChange(), requireRoles(authService, auth.RoleFacilityAdmin, auth.RoleDispatch, auth.RoleFleetManager))
	{
		write.POST("/reservations/hold", h.createHold)
		write.POST("/reservations/:id/confirm", h.confirmReservation)
		write.POST("/reservations/:id/cancel", h.cancelReservation)
	}
}

func (h *reservationHandler) createHold(c *gin.Context) {
	var b struct {
		ZoneID          string `json:"zone_id"`
		MemberID        string `json:"member_id"`
		VehicleID       string `json:"vehicle_id"`
		RatePlanID      string `json:"rate_plan_id"`
		TimeWindowStart string `json:"time_window_start"`
		TimeWindowEnd   string `json:"time_window_end"`
		StallCount      int    `json:"stall_count"`
	}
	if c.ShouldBindJSON(&b) != nil || b.ZoneID == "" || b.MemberID == "" || b.VehicleID == "" || b.StallCount <= 0 {
		abortAPIError(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}
	start, end, ok := parseTimeWindow(c, b.TimeWindowStart, b.TimeWindowEnd)
	if !ok {
		return
	}

	actor, _ := getCurrentUser(c)
	if !h.assertFleetMemberVehicleScope(c, actor, b.MemberID, b.VehicleID) {
		return
	}

	now := time.Now().UTC()
	tx, err := h.pool.Begin(c.Request.Context())
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	defer tx.Rollback(c.Request.Context())

	total, timeoutMins, err := h.lockZone(c, tx, b.ZoneID)
	if err != nil {
		return
	}
	if err := h.releaseExpiredHolds(c, tx, b.ZoneID, now); err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}

	confirmed, held, err := h.usageForWindow(c, tx, b.ZoneID, start, end, now, "")
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	if total-(confirmed+held) < b.StallCount {
		abortAPIError(c, http.StatusConflict, "CAPACITY_CONFLICT", "insufficient stalls for requested window")
		return
	}

	var reservationID string
	err = tx.QueryRow(c.Request.Context(), `
		INSERT INTO reservations(zone_id, member_id, vehicle_id, status, time_window_start, time_window_end, stall_count, rate_plan_id)
		VALUES ($1,$2,$3,'hold',$4,$5,$6, NULLIF($7,'')::uuid)
		RETURNING id::text
	`, b.ZoneID, b.MemberID, b.VehicleID, start, end, b.StallCount, b.RatePlanID).Scan(&reservationID)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}

	expiresAt := now.Add(time.Duration(timeoutMins) * time.Minute)
	_, err = tx.Exec(c.Request.Context(), `
		INSERT INTO capacity_holds(reservation_id, zone_id, stall_count, expires_at)
		VALUES ($1,$2,$3,$4)
	`, reservationID, b.ZoneID, b.StallCount, expiresAt)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}

	if err := h.writeBookingEvent(c, tx, reservationID, "hold_created", map[string]any{"stall_count": b.StallCount, "expires_at": expiresAt.UTC().Format(timeRFC3339)}); err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	if err := h.writeSnapshot(c, tx, b.ZoneID, now); err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}

	if err := tx.Commit(c.Request.Context()); err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":                reservationID,
		"status":            "hold",
		"hold_expires_at":   expiresAt.UTC().Format(timeRFC3339),
		"available_stalls":  total - (confirmed + held + b.StallCount),
		"time_window_start": start.UTC().Format(timeRFC3339),
		"time_window_end":   end.UTC().Format(timeRFC3339),
	})
}

func (h *reservationHandler) confirmReservation(c *gin.Context) {
	now := time.Now().UTC()
	tx, err := h.pool.Begin(c.Request.Context())
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	defer tx.Rollback(c.Request.Context())

	res, err := h.lockReservation(c, tx, c.Param("id"))
	if err != nil {
		return
	}
	if !h.assertReservationFleetScope(c, res.MemberOrgID) {
		return
	}

	if _, _, err := h.lockZone(c, tx, res.ZoneID); err != nil {
		return
	}
	if err := h.releaseExpiredHolds(c, tx, res.ZoneID, now); err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}

	res, err = h.lockReservation(c, tx, c.Param("id"))
	if err != nil {
		return
	}
	if res.Status == "expired" {
		abortAPIError(c, http.StatusConflict, "HOLD_EXPIRED", "reservation hold has expired")
		return
	}
	if res.Status != "hold" {
		abortAPIError(c, http.StatusConflict, "CONFLICT", "reservation is not in hold status")
		return
	}
	if !res.HoldExpiresAt.After(now) || res.HoldReleasedAt != nil {
		_ = h.expireReservationNow(c, tx, res.ID, now)
		abortAPIError(c, http.StatusConflict, "HOLD_EXPIRED", "reservation hold has expired")
		return
	}

	zoneTotal, _, err := h.lockZone(c, tx, res.ZoneID)
	if err != nil {
		return
	}
	confirmed, _, err := h.usageForWindow(c, tx, res.ZoneID, res.TimeWindowStart, res.TimeWindowEnd, now, res.ID)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	if zoneTotal-confirmed < res.StallCount {
		abortAPIError(c, http.StatusConflict, "CAPACITY_CONFLICT", "insufficient stalls for confirmation")
		return
	}

	_, err = tx.Exec(c.Request.Context(), `UPDATE reservations SET status='confirmed', confirmed_at=$2, updated_at=$2 WHERE id=$1`, res.ID, now)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	_, err = tx.Exec(c.Request.Context(), `UPDATE capacity_holds SET released_at=$2 WHERE reservation_id=$1 AND released_at IS NULL`, res.ID, now)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	if err := h.writeBookingEvent(c, tx, res.ID, "confirmed", map[string]any{"stall_count": res.StallCount}); err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	if err := h.writeBookingEvent(c, tx, res.ID, "hold_released", map[string]any{"reason": "confirmed"}); err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	if err := h.writeSnapshot(c, tx, res.ZoneID, now); err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}

	if err := tx.Commit(c.Request.Context()); err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}

	c.JSON(http.StatusOK, gin.H{"id": res.ID, "status": "confirmed", "confirmed_at": now.UTC().Format(timeRFC3339)})
}

func (h *reservationHandler) cancelReservation(c *gin.Context) {
	now := time.Now().UTC()
	tx, err := h.pool.Begin(c.Request.Context())
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	defer tx.Rollback(c.Request.Context())

	res, err := h.lockReservation(c, tx, c.Param("id"))
	if err != nil {
		return
	}
	if !h.assertReservationFleetScope(c, res.MemberOrgID) {
		return
	}

	if _, _, err := h.lockZone(c, tx, res.ZoneID); err != nil {
		return
	}
	if err := h.releaseExpiredHolds(c, tx, res.ZoneID, now); err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}

	res, err = h.lockReservation(c, tx, c.Param("id"))
	if err != nil {
		return
	}
	if res.Status == "cancelled" {
		if err := tx.Commit(c.Request.Context()); err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
		c.JSON(http.StatusOK, gin.H{"id": res.ID, "status": "cancelled"})
		return
	}
	if res.Status == "expired" {
		abortAPIError(c, http.StatusConflict, "CONFLICT", "expired reservation cannot be cancelled")
		return
	}

	_, err = tx.Exec(c.Request.Context(), `UPDATE reservations SET status='cancelled', cancelled_at=$2, updated_at=$2 WHERE id=$1`, res.ID, now)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	_, err = tx.Exec(c.Request.Context(), `UPDATE capacity_holds SET released_at=$2 WHERE reservation_id=$1 AND released_at IS NULL`, res.ID, now)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	if err := h.writeBookingEvent(c, tx, res.ID, "cancelled", map[string]any{"previous_status": res.Status}); err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	if res.Status == "hold" {
		if err := h.writeBookingEvent(c, tx, res.ID, "hold_released", map[string]any{"reason": "cancelled"}); err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
	}
	if err := h.writeSnapshot(c, tx, res.ZoneID, now); err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}

	if err := tx.Commit(c.Request.Context()); err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}

	c.JSON(http.StatusOK, gin.H{"id": res.ID, "status": "cancelled", "cancelled_at": now.UTC().Format(timeRFC3339)})
}

func (h *reservationHandler) getAvailability(c *gin.Context) {
	zoneID := strings.TrimSpace(c.Query("zone_id"))
	if zoneID == "" {
		abortAPIError(c, http.StatusBadRequest, "VALIDATION_ERROR", "zone_id is required")
		return
	}
	start, end, ok := parseTimeWindow(c, c.Query("time_window_start"), c.Query("time_window_end"))
	if !ok {
		return
	}

	now := time.Now().UTC()
	tx, err := h.pool.Begin(c.Request.Context())
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	defer tx.Rollback(c.Request.Context())

	total, _, err := h.lockZone(c, tx, zoneID)
	if err != nil {
		return
	}
	if err := h.releaseExpiredHolds(c, tx, zoneID, now); err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	confirmed, held, err := h.usageForWindow(c, tx, zoneID, start, end, now, "")
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	available := total - (confirmed + held)
	if available < 0 {
		available = 0
	}

	if err := tx.Commit(c.Request.Context()); err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"zone_id":           zoneID,
		"time_window_start": start.UTC().Format(timeRFC3339),
		"time_window_end":   end.UTC().Format(timeRFC3339),
		"total_stalls":      total,
		"held_stalls":       held,
		"confirmed_stalls":  confirmed,
		"available_stalls":  available,
	})
}

func (h *reservationHandler) capacityDashboard(c *gin.Context) {
	now := time.Now().UTC()
	rows, err := h.pool.Query(c.Request.Context(), `
		SELECT
			z.id::text,
			z.name,
			z.total_stalls,
			COALESCE((
				SELECT SUM(r.stall_count)
				FROM reservations r
				WHERE r.zone_id = z.id
				  AND r.status = 'confirmed'
				  AND r.time_window_start <= $1
				  AND r.time_window_end > $1
			), 0) AS confirmed_stalls,
			COALESCE((
				SELECT SUM(h.stall_count)
				FROM capacity_holds h
				JOIN reservations r ON r.id = h.reservation_id
				WHERE h.zone_id = z.id
				  AND h.released_at IS NULL
				  AND h.expires_at > $1
				  AND r.status = 'hold'
				  AND r.time_window_start <= $1
				  AND r.time_window_end > $1
			), 0) AS held_stalls
		FROM zones z
		ORDER BY z.name
	`, now)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	defer rows.Close()

	zones := make([]gin.H, 0)
	for rows.Next() {
		var zoneID, zoneName string
		var total, confirmed, held int
		if err := rows.Scan(&zoneID, &zoneName, &total, &confirmed, &held); err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
		available := total - confirmed - held
		if available < 0 {
			available = 0
		}
		zones = append(zones, gin.H{
			"zone_id":          zoneID,
			"zone_name":        zoneName,
			"total_stalls":     total,
			"confirmed_stalls": confirmed,
			"held_stalls":      held,
			"available_stalls": available,
		})
	}

	c.JSON(http.StatusOK, gin.H{"zones": zones})
}

func (h *reservationHandler) zoneStalls(c *gin.Context) {
	zoneID := c.Param("id")
	start, end, ok := parseTimeWindow(c, c.Query("time_window_start"), c.Query("time_window_end"))
	if !ok {
		return
	}

	now := time.Now().UTC()
	tx, err := h.pool.Begin(c.Request.Context())
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	defer tx.Rollback(c.Request.Context())

	total, _, err := h.lockZone(c, tx, zoneID)
	if err != nil {
		return
	}
	if err := h.releaseExpiredHolds(c, tx, zoneID, now); err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	confirmed, held, err := h.usageForWindow(c, tx, zoneID, start, end, now, "")
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	available := total - (confirmed + held)
	if available < 0 {
		available = 0
	}

	if err := tx.Commit(c.Request.Context()); err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"zone_id":           zoneID,
		"time_window_start": start.UTC().Format(timeRFC3339),
		"time_window_end":   end.UTC().Format(timeRFC3339),
		"total_stalls":      total,
		"held_stalls":       held,
		"confirmed_stalls":  confirmed,
		"available_stalls":  available,
	})
}

func (h *reservationHandler) listSnapshots(c *gin.Context) {
	limit := parseIntDefault(c.Query("limit"), 50)
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := h.pool.Query(c.Request.Context(), `
		SELECT id::text, zone_id::text, snapshot_at, authoritative_stalls
		FROM capacity_snapshots
		ORDER BY snapshot_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	defer rows.Close()
	out := make([]gin.H, 0)
	for rows.Next() {
		var id, zoneID string
		var at time.Time
		var stalls int
		if err := rows.Scan(&id, &zoneID, &at, &stalls); err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
		out = append(out, gin.H{
			"id":                  id,
			"zone_id":             zoneID,
			"snapshot_at":         at.UTC().Format(timeRFC3339),
			"authoritative_stalls": stalls,
		})
	}
	c.JSON(http.StatusOK, out)
}

func (h *reservationHandler) reservationTimeline(c *gin.Context) {
	reservationID := c.Param("id")
	if !h.assertReservationReadScope(c, reservationID) {
		return
	}
	rows, err := h.pool.Query(c.Request.Context(), `
		SELECT event_type, occurred_at, actor_id::text, COALESCE(detail, '{}'::jsonb)
		FROM booking_events
		WHERE reservation_id=$1
		ORDER BY occurred_at ASC
	`, reservationID)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	defer rows.Close()
	out := make([]gin.H, 0)
	for rows.Next() {
		var eventType string
		var at time.Time
		var actorID *string
		var detail []byte
		if err := rows.Scan(&eventType, &at, &actorID, &detail); err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
		payload := map[string]any{}
		_ = json.Unmarshal(detail, &payload)
		out = append(out, gin.H{
			"event_type":  eventType,
			"occurred_at": at.UTC().Format(timeRFC3339),
			"actor_id":    actorID,
			"detail":      payload,
		})
	}
	c.JSON(http.StatusOK, out)
}

func (h *reservationHandler) reservationStatsToday(c *gin.Context) {
	now := time.Now().UTC()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	dayEnd := dayStart.Add(24 * time.Hour)

	var count int
	err := h.pool.QueryRow(c.Request.Context(), `
		SELECT COUNT(*)
		FROM reservations
		WHERE created_at >= $1
		  AND created_at < $2
	`, dayStart, dayEnd).Scan(&count)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	c.JSON(http.StatusOK, gin.H{"total_reservations_today": count})
}

func (h *reservationHandler) listReservations(c *gin.Context) {
	status := strings.TrimSpace(c.Query("status"))
	limit := parseIntDefault(c.Query("limit"), 50)
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	query := `
		SELECT
			r.id::text,
			r.zone_id::text,
			r.member_id::text,
			r.vehicle_id::text,
			r.status,
			r.time_window_start,
			r.time_window_end,
			r.stall_count,
			h.expires_at
		FROM reservations r
		LEFT JOIN capacity_holds h ON h.reservation_id = r.id AND h.released_at IS NULL
	`
	args := []any{}
	if status != "" {
		query += ` WHERE r.status = $1`
		args = append(args, status)
	}
	query += ` ORDER BY r.created_at DESC LIMIT $` + strconv.Itoa(len(args)+1)
	args = append(args, limit)

	rows, err := h.pool.Query(c.Request.Context(), query, args...)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	defer rows.Close()

	items := make([]gin.H, 0)
	for rows.Next() {
		var id, zoneID, memberID, vehicleID, rs string
		var start, end time.Time
		var stallCount int
		var holdExpires *time.Time
		if err := rows.Scan(&id, &zoneID, &memberID, &vehicleID, &rs, &start, &end, &stallCount, &holdExpires); err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
		item := gin.H{
			"id":                id,
			"zone_id":           zoneID,
			"member_id":         memberID,
			"vehicle_id":        vehicleID,
			"status":            rs,
			"time_window_start": start.UTC().Format(timeRFC3339),
			"time_window_end":   end.UTC().Format(timeRFC3339),
			"stall_count":       stallCount,
		}
		if holdExpires != nil {
			item["hold_expires_at"] = holdExpires.UTC().Format(timeRFC3339)
		}
		items = append(items, item)
	}

	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

func (h *reservationHandler) listOpenExceptions(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": []gin.H{}})
}

type lockedReservation struct {
	ID              string
	ZoneID          string
	Status          string
	StallCount      int
	TimeWindowStart time.Time
	TimeWindowEnd   time.Time
	HoldExpiresAt   time.Time
	HoldReleasedAt  *time.Time
	MemberOrgID     string
}

func (h *reservationHandler) lockReservation(c *gin.Context, tx pgx.Tx, reservationID string) (lockedReservation, error) {
	var out lockedReservation
	err := tx.QueryRow(c.Request.Context(), `
		SELECT
			r.id::text,
			r.zone_id::text,
			r.status,
			r.stall_count,
			r.time_window_start,
			r.time_window_end,
			COALESCE(h.expires_at, r.created_at),
			h.released_at,
			m.organization_id::text
		FROM reservations r
		LEFT JOIN capacity_holds h ON h.reservation_id = r.id
		JOIN members m ON m.id = r.member_id
		WHERE r.id = $1
		FOR UPDATE OF r
	`, reservationID).Scan(
		&out.ID,
		&out.ZoneID,
		&out.Status,
		&out.StallCount,
		&out.TimeWindowStart,
		&out.TimeWindowEnd,
		&out.HoldExpiresAt,
		&out.HoldReleasedAt,
		&out.MemberOrgID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			abortAPIError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
			return lockedReservation{}, err
		}
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return lockedReservation{}, err
	}
	return out, nil
}

func (h *reservationHandler) lockZone(c *gin.Context, tx pgx.Tx, zoneID string) (int, int, error) {
	var total, timeoutMins int
	err := tx.QueryRow(c.Request.Context(), `
		SELECT total_stalls, hold_timeout_minutes
		FROM zones
		WHERE id = $1
		FOR UPDATE
	`, zoneID).Scan(&total, &timeoutMins)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			abortAPIError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
			return 0, 0, err
		}
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return 0, 0, err
	}
	if timeoutMins <= 0 {
		timeoutMins = 15
	}
	return total, timeoutMins, nil
}

func (h *reservationHandler) usageForWindow(c *gin.Context, tx pgx.Tx, zoneID string, start, end, now time.Time, excludeReservationID string) (int, int, error) {
	var confirmed int
	err := tx.QueryRow(c.Request.Context(), `
		SELECT COALESCE(SUM(stall_count),0)
		FROM reservations
		WHERE zone_id = $1
		  AND status = 'confirmed'
		  AND time_window_start < $2
		  AND time_window_end > $3
		  AND ($4 = '' OR id::text <> $4)
	`, zoneID, end, start, excludeReservationID).Scan(&confirmed)
	if err != nil {
		return 0, 0, err
	}

	var held int
	err = tx.QueryRow(c.Request.Context(), `
		SELECT COALESCE(SUM(h.stall_count),0)
		FROM capacity_holds h
		JOIN reservations r ON r.id = h.reservation_id
		WHERE h.zone_id = $1
		  AND h.released_at IS NULL
		  AND h.expires_at > $2
		  AND r.status = 'hold'
		  AND r.time_window_start < $3
		  AND r.time_window_end > $4
		  AND ($5 = '' OR r.id::text <> $5)
	`, zoneID, now, end, start, excludeReservationID).Scan(&held)
	if err != nil {
		return 0, 0, err
	}

	return confirmed, held, nil
}

func (h *reservationHandler) releaseExpiredHolds(c *gin.Context, tx pgx.Tx, zoneID string, now time.Time) error {
	rows, err := tx.Query(c.Request.Context(), `
		SELECT h.reservation_id::text
		FROM capacity_holds h
		JOIN reservations r ON r.id = h.reservation_id
		WHERE h.zone_id = $1
		  AND h.released_at IS NULL
		  AND h.expires_at <= $2
		  AND r.status = 'hold'
		FOR UPDATE OF h, r
	`, zoneID, now)
	if err != nil {
		return err
	}
	defer rows.Close()

	expiredIDs := make([]string, 0)
	for rows.Next() {
		var reservationID string
		if err := rows.Scan(&reservationID); err != nil {
			return err
		}
		expiredIDs = append(expiredIDs, reservationID)
	}

	for _, reservationID := range expiredIDs {
		if err := h.expireReservationNow(c, tx, reservationID, now); err != nil {
			return err
		}
	}
	if len(expiredIDs) > 0 {
		if err := h.writeSnapshot(c, tx, zoneID, now); err != nil {
			return err
		}
	}
	return nil
}

func (h *reservationHandler) expireReservationNow(c *gin.Context, tx pgx.Tx, reservationID string, now time.Time) error {
	_, err := tx.Exec(c.Request.Context(), `
		UPDATE reservations
		SET status='expired', updated_at=$2
		WHERE id=$1 AND status='hold'
	`, reservationID, now)
	if err != nil {
		return err
	}
	_, err = tx.Exec(c.Request.Context(), `
		UPDATE capacity_holds
		SET released_at=$2
		WHERE reservation_id=$1 AND released_at IS NULL
	`, reservationID, now)
	if err != nil {
		return err
	}
	if err := h.writeBookingEvent(c, tx, reservationID, "expired", nil); err != nil {
		return err
	}
	if err := h.writeBookingEvent(c, tx, reservationID, "hold_released", map[string]any{"reason": "expired"}); err != nil {
		return err
	}
	return nil
}

func (h *reservationHandler) writeBookingEvent(c *gin.Context, tx pgx.Tx, reservationID, eventType string, detail map[string]any) error {
	var actorID any
	if id := getActorID(c); id != nil {
		actorID = *id
	}
	b, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	_, err = tx.Exec(c.Request.Context(), `
		INSERT INTO booking_events(reservation_id, event_type, occurred_at, actor_id, detail)
		VALUES ($1,$2,$3,$4,COALESCE($5::jsonb, '{}'::jsonb))
	`, reservationID, eventType, time.Now().UTC(), actorID, string(b))
	return err
}

func (h *reservationHandler) writeSnapshot(c *gin.Context, tx pgx.Tx, zoneID string, at time.Time) error {
	var total int
	err := tx.QueryRow(c.Request.Context(), `SELECT total_stalls FROM zones WHERE id=$1`, zoneID).Scan(&total)
	if err != nil {
		return err
	}
	var confirmed int
	err = tx.QueryRow(c.Request.Context(), `
		SELECT COALESCE(SUM(stall_count),0)
		FROM reservations
		WHERE zone_id=$1
		  AND status='confirmed'
		  AND time_window_start <= $2
		  AND time_window_end > $2
	`, zoneID, at).Scan(&confirmed)
	if err != nil {
		return err
	}
	var held int
	err = tx.QueryRow(c.Request.Context(), `
		SELECT COALESCE(SUM(h.stall_count),0)
		FROM capacity_holds h
		JOIN reservations r ON r.id = h.reservation_id
		WHERE h.zone_id=$1
		  AND h.released_at IS NULL
		  AND h.expires_at > $2
		  AND r.status='hold'
		  AND r.time_window_start <= $2
		  AND r.time_window_end > $2
	`, zoneID, at).Scan(&held)
	if err != nil {
		return err
	}
	authoritative := total - confirmed - held
	if authoritative < 0 {
		authoritative = 0
	}
	_, err = tx.Exec(c.Request.Context(), `
		INSERT INTO capacity_snapshots(zone_id, snapshot_at, authoritative_stalls)
		VALUES ($1,$2,$3)
	`, zoneID, at, authoritative)
	return err
}

func (h *reservationHandler) assertFleetMemberVehicleScope(c *gin.Context, actor auth.User, memberID, vehicleID string) bool {
	if !auth.HasAnyRole(actor.Roles, []string{auth.RoleFleetManager}) {
		return true
	}
	if actor.OrganizationID == nil {
		abortAPIError(c, http.StatusForbidden, "FORBIDDEN", "forbidden")
		return false
	}
	var memberOrg, vehicleOrg string
	err := h.pool.QueryRow(c.Request.Context(), `SELECT organization_id::text FROM members WHERE id=$1`, memberID).Scan(&memberOrg)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			abortAPIError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
		} else {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		}
		return false
	}
	err = h.pool.QueryRow(c.Request.Context(), `SELECT organization_id::text FROM vehicles WHERE id=$1`, vehicleID).Scan(&vehicleOrg)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			abortAPIError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
		} else {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		}
		return false
	}
	if memberOrg != *actor.OrganizationID || vehicleOrg != *actor.OrganizationID {
		abortAPIError(c, http.StatusForbidden, "FORBIDDEN", "forbidden")
		return false
	}
	return true
}

func (h *reservationHandler) assertReservationFleetScope(c *gin.Context, memberOrgID string) bool {
	user, ok := getCurrentUser(c)
	if !ok {
		abortAPIError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return false
	}
	if !auth.HasAnyRole(user.Roles, []string{auth.RoleFleetManager}) {
		return true
	}
	if user.OrganizationID == nil || *user.OrganizationID != memberOrgID {
		abortAPIError(c, http.StatusForbidden, "FORBIDDEN", "forbidden")
		return false
	}
	return true
}

func (h *reservationHandler) assertReservationReadScope(c *gin.Context, reservationID string) bool {
	user, ok := getCurrentUser(c)
	if !ok {
		abortAPIError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return false
	}
	if !auth.HasAnyRole(user.Roles, []string{auth.RoleFleetManager}) {
		return true
	}
	if user.OrganizationID == nil {
		abortAPIError(c, http.StatusForbidden, "FORBIDDEN", "forbidden")
		return false
	}
	var memberOrg string
	err := h.pool.QueryRow(c.Request.Context(), `
		SELECT m.organization_id::text
		FROM reservations r
		JOIN members m ON m.id = r.member_id
		WHERE r.id=$1
	`, reservationID).Scan(&memberOrg)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			abortAPIError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
		} else {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		}
		return false
	}
	if memberOrg != *user.OrganizationID {
		abortAPIError(c, http.StatusForbidden, "FORBIDDEN", "forbidden")
		return false
	}
	return true
}

func parseTimeWindow(c *gin.Context, rawStart, rawEnd string) (time.Time, time.Time, bool) {
	start, err := time.Parse(time.RFC3339, strings.TrimSpace(rawStart))
	if err != nil {
		abortAPIError(c, http.StatusBadRequest, "VALIDATION_ERROR", "time_window_start must be RFC3339")
		return time.Time{}, time.Time{}, false
	}
	end, err := time.Parse(time.RFC3339, strings.TrimSpace(rawEnd))
	if err != nil {
		abortAPIError(c, http.StatusBadRequest, "VALIDATION_ERROR", "time_window_end must be RFC3339")
		return time.Time{}, time.Time{}, false
	}
	if !end.After(start) {
		abortAPIError(c, http.StatusBadRequest, "VALIDATION_ERROR", "time_window_end must be after start")
		return time.Time{}, time.Time{}, false
	}
	return start.UTC(), end.UTC(), true
}

func maxConfirmedDemand(ctx *gin.Context, pool *pgxpool.Pool, zoneID string) (int, error) {
	var maxDemand int
	err := pool.QueryRow(ctx.Request.Context(), `
		WITH events AS (
			SELECT time_window_start AS ts, stall_count AS delta
			FROM reservations
			WHERE zone_id = $1 AND status = 'confirmed'
			UNION ALL
			SELECT time_window_end AS ts, -stall_count AS delta
			FROM reservations
			WHERE zone_id = $1 AND status = 'confirmed'
		), ordered AS (
			SELECT ts, delta,
				SUM(delta) OVER (ORDER BY ts, delta ASC ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW) AS running
			FROM events
		)
		SELECT COALESCE(MAX(running), 0)
		FROM ordered
	`, zoneID).Scan(&maxDemand)
	if err != nil {
		return 0, err
	}
	return maxDemand, nil
}
