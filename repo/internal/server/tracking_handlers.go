package server

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"parkops/internal/auth"
	"parkops/internal/tracking"
)

type trackingHandler struct {
	pool *pgxpool.Pool
}

func registerTrackingRoutes(r *gin.Engine, authService *auth.Service, pool *pgxpool.Pool) {
	h := &trackingHandler{pool: pool}

	read := r.Group("/api")
	read.Use(requireSession(authService), enforceForcePasswordChange(), requireRoles(authService, allSystemRoles()...))
	{
		read.GET("/tracking/vehicles/:id/positions", h.listVehiclePositions)
		read.GET("/tracking/vehicles/:id/stops", h.listVehicleStops)
	}

	write := r.Group("/api")
	write.Use(requireSession(authService), enforceForcePasswordChange(), requireRoles(authService, auth.RoleFacilityAdmin, auth.RoleDispatch, auth.RoleFleetManager))
	{
		write.POST("/tracking/location", h.submitLocation)
	}
}

type positionRow struct {
	ID         string
	Latitude   float64
	Longitude  float64
	ReceivedAt time.Time
}

func (h *trackingHandler) submitLocation(c *gin.Context) {
	actor, ok := getCurrentUser(c)
	if !ok {
		abortAPIError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	var b struct {
		VehicleID           string  `json:"vehicle_id"`
		Latitude            float64 `json:"latitude"`
		Longitude           float64 `json:"longitude"`
		DeviceTime          string  `json:"device_time"`
		DeviceTimeSignature string  `json:"device_time_signature"`
	}
	if c.ShouldBindJSON(&b) != nil || strings.TrimSpace(b.VehicleID) == "" {
		abortAPIError(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}
	if b.Latitude < -90 || b.Latitude > 90 || b.Longitude < -180 || b.Longitude > 180 {
		abortAPIError(c, http.StatusBadRequest, "VALIDATION_ERROR", "latitude/longitude out of range")
		return
	}

	now := time.Now().UTC()
	var deviceTime *time.Time
	if strings.TrimSpace(b.DeviceTime) != "" {
		parsed, err := time.Parse(time.RFC3339, b.DeviceTime)
		if err != nil {
			abortAPIError(c, http.StatusBadRequest, "VALIDATION_ERROR", "device_time must be RFC3339")
			return
		}
		u := parsed.UTC()
		deviceTime = &u
	}

	tx, err := h.pool.Begin(c.Request.Context())
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	defer tx.Rollback(c.Request.Context())

	var plateNumber string
	var vehicleOrg string
	err = tx.QueryRow(c.Request.Context(), `SELECT plate_number FROM vehicles WHERE id=$1`, b.VehicleID).Scan(&plateNumber)
	if err == nil {
		err = tx.QueryRow(c.Request.Context(), `SELECT organization_id::text FROM vehicles WHERE id=$1`, b.VehicleID).Scan(&vehicleOrg)
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			abortAPIError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
			return
		}
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	if auth.HasAnyRole(actor.Roles, []string{auth.RoleFleetManager}) {
		if actor.OrganizationID == nil || *actor.OrganizationID != vehicleOrg {
			abortAPIError(c, http.StatusForbidden, "FORBIDDEN", "forbidden")
			return
		}
	}

	deviceTimeTrusted := false
	if deviceTime != nil && strings.TrimSpace(b.DeviceTimeSignature) != "" {
		deviceTimeTrusted = tracking.ValidateDeviceTimeHMAC(b.DeviceTime, b.DeviceTimeSignature, plateNumber)
	}

	pending, hasPending, err := h.lockPendingSuspect(c, tx, b.VehicleID)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	lastConfirmed, hasConfirmed, err := h.lockLastConfirmedPosition(c, tx, b.VehicleID)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}

	if hasPending {
		if tracking.ConfirmsSuspect(pending.Latitude, pending.Longitude, b.Latitude, b.Longitude) {
			if _, err := tx.Exec(c.Request.Context(), `UPDATE vehicle_positions SET confirmed=true WHERE id=$1`, pending.ID); err != nil {
				abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
				return
			}
			id, err := h.insertPosition(c, tx, b.VehicleID, b.Latitude, b.Longitude, now, deviceTime, deviceTimeTrusted, false, true)
			if err != nil {
				abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
				return
			}
			if err := h.maybeCreateStopEvent(c, tx, b.VehicleID); err != nil {
				abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
				return
			}
			if err := tx.Commit(c.Request.Context()); err != nil {
				abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
				return
			}
			c.JSON(http.StatusCreated, gin.H{"id": id, "suspect": false, "confirmed": true, "device_time_trusted": deviceTimeTrusted})
			return
		}

		if _, err := tx.Exec(c.Request.Context(), `UPDATE vehicle_positions SET discarded_at=$2 WHERE id=$1`, pending.ID, now); err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
	}

	if hasConfirmed && tracking.IsSuspectJump(lastConfirmed.ReceivedAt, now, lastConfirmed.Latitude, lastConfirmed.Longitude, b.Latitude, b.Longitude) {
		id, err := h.insertPosition(c, tx, b.VehicleID, b.Latitude, b.Longitude, now, deviceTime, deviceTimeTrusted, true, false)
		if err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
		if err := tx.Commit(c.Request.Context()); err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
		c.JSON(http.StatusCreated, gin.H{"id": id, "suspect": true, "confirmed": false, "device_time_trusted": deviceTimeTrusted})
		return
	}

	id, err := h.insertPosition(c, tx, b.VehicleID, b.Latitude, b.Longitude, now, deviceTime, deviceTimeTrusted, false, true)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	if err := h.maybeCreateStopEvent(c, tx, b.VehicleID); err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	if err := tx.Commit(c.Request.Context()); err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": id, "suspect": false, "confirmed": true, "device_time_trusted": deviceTimeTrusted})
}

func (h *trackingHandler) insertPosition(c *gin.Context, tx pgx.Tx, vehicleID string, lat, lon float64, receivedAt time.Time, deviceTime *time.Time, trusted, suspect, confirmed bool) (string, error) {
	var id string
	err := tx.QueryRow(c.Request.Context(), `
		INSERT INTO vehicle_positions(vehicle_id, latitude, longitude, received_at, device_time, device_time_trusted, suspect, confirmed)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id::text
	`, vehicleID, lat, lon, receivedAt, deviceTime, trusted, suspect, confirmed).Scan(&id)
	return id, err
}

func (h *trackingHandler) lockPendingSuspect(c *gin.Context, tx pgx.Tx, vehicleID string) (positionRow, bool, error) {
	var row positionRow
	err := tx.QueryRow(c.Request.Context(), `
		SELECT id::text, latitude, longitude, received_at
		FROM vehicle_positions
		WHERE vehicle_id = $1
		  AND suspect = true
		  AND confirmed = false
		  AND discarded_at IS NULL
		ORDER BY received_at DESC
		LIMIT 1
		FOR UPDATE
	`, vehicleID).Scan(&row.ID, &row.Latitude, &row.Longitude, &row.ReceivedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return positionRow{}, false, nil
	}
	if err != nil {
		return positionRow{}, false, err
	}
	return row, true, nil
}

func (h *trackingHandler) lockLastConfirmedPosition(c *gin.Context, tx pgx.Tx, vehicleID string) (positionRow, bool, error) {
	var row positionRow
	err := tx.QueryRow(c.Request.Context(), `
		SELECT id::text, latitude, longitude, received_at
		FROM vehicle_positions
		WHERE vehicle_id = $1
		  AND confirmed = true
		  AND discarded_at IS NULL
		ORDER BY received_at DESC
		LIMIT 1
		FOR UPDATE
	`, vehicleID).Scan(&row.ID, &row.Latitude, &row.Longitude, &row.ReceivedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return positionRow{}, false, nil
	}
	if err != nil {
		return positionRow{}, false, err
	}
	return row, true, nil
}

func (h *trackingHandler) maybeCreateStopEvent(c *gin.Context, tx pgx.Tx, vehicleID string) error {
	rows, err := tx.Query(c.Request.Context(), `
		SELECT latitude, longitude, received_at
		FROM vehicle_positions
		WHERE vehicle_id = $1
		  AND confirmed = true
		  AND discarded_at IS NULL
		ORDER BY received_at DESC
		LIMIT 200
	`, vehicleID)
	if err != nil {
		return err
	}
	defer rows.Close()

	positions := make([]positionRow, 0)
	for rows.Next() {
		var p positionRow
		if err := rows.Scan(&p.Latitude, &p.Longitude, &p.ReceivedAt); err != nil {
			return err
		}
		positions = append(positions, p)
	}
	if len(positions) < 2 {
		return nil
	}

	current := positions[0]
	stationaryStart := current.ReceivedAt
	for i := 1; i < len(positions); i++ {
		d := tracking.DistanceMeters(current.Latitude, current.Longitude, positions[i].Latitude, positions[i].Longitude)
		if d > tracking.StopDistanceMeters {
			break
		}
		stationaryStart = positions[i].ReceivedAt
	}

	duration := current.ReceivedAt.Sub(stationaryStart)
	if !tracking.ShouldCreateStop(duration, 0) {
		return nil
	}

	var existing string
	err = tx.QueryRow(c.Request.Context(), `SELECT id::text FROM stop_events WHERE vehicle_id=$1 AND started_at=$2 LIMIT 1`, vehicleID, stationaryStart).Scan(&existing)
	if err == nil {
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	_, err = tx.Exec(c.Request.Context(), `
		INSERT INTO stop_events(vehicle_id, started_at, detected_at, latitude, longitude)
		VALUES ($1, $2, $3, $4, $5)
	`, vehicleID, stationaryStart, current.ReceivedAt, current.Latitude, current.Longitude)
	return err
}

func (h *trackingHandler) listVehiclePositions(c *gin.Context) {
	if !h.assertTrackingVehicleScope(c, c.Param("id")) {
		return
	}

	vehicleID := strings.TrimSpace(c.Param("id"))
	if vehicleID == "" {
		abortAPIError(c, http.StatusBadRequest, "VALIDATION_ERROR", "vehicle id is required")
		return
	}

	args := make([]any, 0, 4)
	clauses := []string{"vehicle_id = $1", "discarded_at IS NULL"}
	args = append(args, vehicleID)

	if fromRaw := strings.TrimSpace(c.Query("from")); fromRaw != "" {
		from, err := time.Parse(time.RFC3339, fromRaw)
		if err != nil {
			abortAPIError(c, http.StatusBadRequest, "VALIDATION_ERROR", "from must be RFC3339")
			return
		}
		args = append(args, from.UTC())
		clauses = append(clauses, "received_at >= $"+strconv.Itoa(len(args)))
	}
	if toRaw := strings.TrimSpace(c.Query("to")); toRaw != "" {
		to, err := time.Parse(time.RFC3339, toRaw)
		if err != nil {
			abortAPIError(c, http.StatusBadRequest, "VALIDATION_ERROR", "to must be RFC3339")
			return
		}
		args = append(args, to.UTC())
		clauses = append(clauses, "received_at <= $"+strconv.Itoa(len(args)))
	}
	if suspectRaw := strings.TrimSpace(c.Query("suspect")); suspectRaw != "" {
		suspect := strings.EqualFold(suspectRaw, "true")
		args = append(args, suspect)
		clauses = append(clauses, "suspect = $"+strconv.Itoa(len(args)))
	}

	query := `
		SELECT id::text, latitude, longitude, received_at, device_time, device_time_trusted, suspect, confirmed
		FROM vehicle_positions
		WHERE ` + strings.Join(clauses, " AND ") + `
		ORDER BY received_at DESC
		LIMIT 200
	`

	rows, err := h.pool.Query(c.Request.Context(), query, args...)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	defer rows.Close()

	items := make([]gin.H, 0)
	for rows.Next() {
		var id string
		var lat, lon float64
		var receivedAt time.Time
		var deviceTime *time.Time
		var trusted, suspect, confirmed bool
		if err := rows.Scan(&id, &lat, &lon, &receivedAt, &deviceTime, &trusted, &suspect, &confirmed); err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
		item := gin.H{
			"id":                  id,
			"latitude":            lat,
			"longitude":           lon,
			"received_at":         receivedAt.UTC().Format(timeRFC3339),
			"device_time_trusted": trusted,
			"suspect":             suspect,
			"confirmed":           confirmed,
		}
		if deviceTime != nil {
			item["device_time"] = deviceTime.UTC().Format(timeRFC3339)
		}
		items = append(items, item)
	}

	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

func (h *trackingHandler) listVehicleStops(c *gin.Context) {
	if !h.assertTrackingVehicleScope(c, c.Param("id")) {
		return
	}

	rows, err := h.pool.Query(c.Request.Context(), `
		SELECT id::text, started_at, detected_at, latitude, longitude
		FROM stop_events
		WHERE vehicle_id = $1
		ORDER BY detected_at DESC
		LIMIT 200
	`, c.Param("id"))
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	defer rows.Close()

	items := make([]gin.H, 0)
	for rows.Next() {
		var id string
		var startedAt, detectedAt time.Time
		var lat, lon *float64
		if err := rows.Scan(&id, &startedAt, &detectedAt, &lat, &lon); err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
		item := gin.H{
			"id":          id,
			"started_at":  startedAt.UTC().Format(timeRFC3339),
			"detected_at": detectedAt.UTC().Format(timeRFC3339),
		}
		if lat != nil {
			item["latitude"] = *lat
		}
		if lon != nil {
			item["longitude"] = *lon
		}
		items = append(items, item)
	}

	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

func (h *trackingHandler) assertTrackingVehicleScope(c *gin.Context, vehicleID string) bool {
	actor, ok := getCurrentUser(c)
	if !ok {
		abortAPIError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return false
	}
	if !auth.HasAnyRole(actor.Roles, []string{auth.RoleFleetManager}) {
		return true
	}
	if actor.OrganizationID == nil {
		abortAPIError(c, http.StatusForbidden, "FORBIDDEN", "forbidden")
		return false
	}

	var vehicleOrg string
	err := h.pool.QueryRow(c.Request.Context(), `SELECT organization_id::text FROM vehicles WHERE id=$1`, vehicleID).Scan(&vehicleOrg)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			abortAPIError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
		} else {
			abortAPIError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		}
		return false
	}
	if vehicleOrg != *actor.OrganizationID {
		abortAPIError(c, http.StatusForbidden, "FORBIDDEN", "forbidden")
		return false
	}
	return true
}
