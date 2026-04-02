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
	"parkops/internal/devices"
	"parkops/internal/exceptions"
	"parkops/internal/tracking"
)

const reorderWindow = 10 * time.Minute

type deviceHandler struct {
	pool        *pgxpool.Pool
	authService *auth.Service
}

func registerDeviceRoutes(r *gin.Engine, authService *auth.Service, pool *pgxpool.Pool) {
	h := &deviceHandler{pool: pool, authService: authService}

	read := r.Group("/api")
	read.Use(requireSession(authService), enforceForcePasswordChange(), requireRoles(authService, allSystemRoles()...))
	{
		read.GET("/devices", h.listDevices)
		read.GET("/devices/:id", h.getDevice)
		read.GET("/device-events", h.listDeviceEvents)
	}

	write := r.Group("/api")
	write.Use(requireSession(authService), enforceForcePasswordChange(), requireRoles(authService, auth.RoleFacilityAdmin, auth.RoleDispatch))
	{
		write.POST("/device-events", h.ingestDeviceEvent)
		write.POST("/device-events/replay", h.replayDeviceEvents)
	}

	admin := r.Group("/api")
	admin.Use(requireSession(authService), enforceForcePasswordChange(), requireRoles(authService, auth.RoleFacilityAdmin))
	{
		admin.POST("/devices", h.registerDevice)
		admin.PATCH("/devices/:id", h.updateDevice)
		admin.DELETE("/devices/:id", h.deleteDevice)
	}
}

func (h *deviceHandler) listDevices(c *gin.Context) {
	user, ok := getCurrentUser(c)
	if !ok {
		abortAPIError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	query := `
		SELECT id::text, organization_id::text, device_key, device_type, zone_id::text, status, registered_at
		FROM devices
	`
	args := []any{}
	if auth.HasAnyRole(user.Roles, []string{auth.RoleFleetManager}) {
		if user.OrganizationID == nil {
			abortAPIError(c, http.StatusForbidden, "FORBIDDEN", "forbidden")
			return
		}
		query += ` WHERE organization_id=$1::uuid`
		args = append(args, *user.OrganizationID)
	}
	query += ` ORDER BY registered_at DESC`

	rows, err := h.pool.Query(c.Request.Context(), query, args...)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	defer rows.Close()
	out := make([]gin.H, 0)
	for rows.Next() {
		var id, key, typ, status string
		var orgID, zoneID *string
		var registeredAt time.Time
		if err := rows.Scan(&id, &orgID, &key, &typ, &zoneID, &status, &registeredAt); err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
		out = append(out, gin.H{
			"id":            id,
			"organization_id": orgID,
			"device_key":    key,
			"device_type":   typ,
			"zone_id":       zoneID,
			"status":        status,
			"registered_at": registeredAt.UTC().Format(timeRFC3339),
		})
	}
	c.JSON(http.StatusOK, out)
}

func (h *deviceHandler) getDevice(c *gin.Context) {
	user, ok := getCurrentUser(c)
	if !ok {
		abortAPIError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	var id, key, typ, status string
	var orgID, zoneID *string
	var registeredAt time.Time
	err := h.pool.QueryRow(c.Request.Context(), `
		SELECT id::text, organization_id::text, device_key, device_type, zone_id::text, status, registered_at
		FROM devices
		WHERE id=$1
	`, c.Param("id")).Scan(&id, &orgID, &key, &typ, &zoneID, &status, &registeredAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			abortAPIError(c, 404, "NOT_FOUND", "resource not found")
			return
		}
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	if auth.HasAnyRole(user.Roles, []string{auth.RoleFleetManager}) {
		if user.OrganizationID == nil || orgID == nil || *user.OrganizationID != *orgID {
			abortAPIError(c, http.StatusForbidden, "FORBIDDEN", "forbidden")
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"id":              id,
		"organization_id": orgID,
		"device_key":      key,
		"device_type":     typ,
		"zone_id":         zoneID,
		"status":          status,
		"registered_at":   registeredAt.UTC().Format(timeRFC3339),
	})
}

func (h *deviceHandler) updateDevice(c *gin.Context) {
	var b struct {
		ZoneID  string `json:"zone_id"`
		Status  string `json:"status"`
		OrgID   string `json:"organization_id"`
	}
	if c.ShouldBindJSON(&b) != nil {
		abortAPIError(c, 400, "VALIDATION_ERROR", "invalid request body")
		return
	}
	status := strings.TrimSpace(strings.ToLower(b.Status))
	if status != "" && status != "online" && status != "offline" {
		abortAPIError(c, 400, "VALIDATION_ERROR", "status must be online|offline")
		return
	}
	if status == "" {
		ct, err := h.pool.Exec(c.Request.Context(), `
			UPDATE devices
			SET zone_id = NULLIF($2,'')::uuid,
			    organization_id = NULLIF($3,'')::uuid
			WHERE id = $1
		`, c.Param("id"), b.ZoneID, b.OrgID)
		if err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
		if ct.RowsAffected() == 0 {
			abortAPIError(c, 404, "NOT_FOUND", "resource not found")
			return
		}
	} else {
		ct, err := h.pool.Exec(c.Request.Context(), `
			UPDATE devices
			SET zone_id = NULLIF($2,'')::uuid,
			    organization_id = NULLIF($3,'')::uuid,
			    status = $4
			WHERE id = $1
		`, c.Param("id"), b.ZoneID, b.OrgID, status)
		if err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
		if ct.RowsAffected() == 0 {
			abortAPIError(c, 404, "NOT_FOUND", "resource not found")
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func (h *deviceHandler) deleteDevice(c *gin.Context) {
	_, err := h.pool.Exec(c.Request.Context(), `DELETE FROM devices WHERE id=$1`, c.Param("id"))
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *deviceHandler) registerDevice(c *gin.Context) {
	var b struct {
		OrganizationID string `json:"organization_id"`
		DeviceKey      string `json:"device_key"`
		DeviceType     string `json:"device_type"`
		ZoneID         string `json:"zone_id"`
		Status         string `json:"status"`
	}
	if c.ShouldBindJSON(&b) != nil || strings.TrimSpace(b.DeviceKey) == "" || strings.TrimSpace(b.DeviceType) == "" {
		abortAPIError(c, 400, "VALIDATION_ERROR", "invalid request body")
		return
	}
	b.DeviceType = strings.TrimSpace(strings.ToLower(b.DeviceType))
	if b.DeviceType != "camera" && b.DeviceType != "gate" && b.DeviceType != "geomagnetic" {
		abortAPIError(c, 400, "VALIDATION_ERROR", "device_type must be camera|gate|geomagnetic")
		return
	}
	b.Status = strings.TrimSpace(strings.ToLower(b.Status))
	if b.Status == "" {
		b.Status = "online"
	}
	if b.Status != "online" && b.Status != "offline" {
		abortAPIError(c, 400, "VALIDATION_ERROR", "status must be online|offline")
		return
	}

	var id string
	err := h.pool.QueryRow(c.Request.Context(), `
		INSERT INTO devices(organization_id, device_key, device_type, zone_id, status)
		VALUES (NULLIF($1,'')::uuid, $2, $3, NULLIF($4,'')::uuid, $5)
		RETURNING id::text
	`, b.OrganizationID, b.DeviceKey, b.DeviceType, b.ZoneID, b.Status).Scan(&id)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func (h *deviceHandler) ingestDeviceEvent(c *gin.Context) {
	var b struct {
		DeviceID            string         `json:"device_id"`
		EventKey            string         `json:"event_key"`
		SequenceNumber      int64          `json:"sequence_number"`
		EventType           string         `json:"event_type"`
		Payload             map[string]any `json:"payload"`
		DeviceTime          string         `json:"device_time"`
		DeviceTimeSignature string         `json:"device_time_signature"`
	}
	if c.ShouldBindJSON(&b) != nil || strings.TrimSpace(b.DeviceID) == "" || strings.TrimSpace(b.EventKey) == "" || b.SequenceNumber <= 0 || strings.TrimSpace(b.EventType) == "" {
		abortAPIError(c, 400, "VALIDATION_ERROR", "missing required device event fields")
		return
	}
	now := time.Now().UTC()
	var deviceTime *time.Time
	if strings.TrimSpace(b.DeviceTime) != "" {
		t, err := time.Parse(time.RFC3339, b.DeviceTime)
		if err != nil {
			abortAPIError(c, 400, "VALIDATION_ERROR", "device_time must be RFC3339")
			return
		}
		u := t.UTC()
		deviceTime = &u
	}

	tx, err := h.pool.Begin(c.Request.Context())
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	defer tx.Rollback(c.Request.Context())

	var existingID string
	err = tx.QueryRow(c.Request.Context(), `SELECT id::text FROM device_events WHERE event_key=$1`, b.EventKey).Scan(&existingID)
	if err == nil {
		if err := tx.Commit(c.Request.Context()); err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "already_processed"})
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}

	var lastApplied int64
	var lastSequence int64
	var lastSeenAt *time.Time
	var deviceKey string
	err = tx.QueryRow(c.Request.Context(), `
		SELECT last_applied_sequence_number, last_sequence_number, last_event_received_at, device_key
		FROM devices
		WHERE id=$1
		FOR UPDATE
	`, b.DeviceID).Scan(&lastApplied, &lastSequence, &lastSeenAt, &deviceKey)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			abortAPIError(c, 403, "FORBIDDEN", "device is not registered")
			return
		}
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}

	var seenAt time.Time
	if lastSeenAt != nil {
		seenAt = *lastSeenAt
	}
	late, reordered := devices.ClassifySequence(lastSequence, seenAt, b.SequenceNumber, now, reorderWindow)

	payload, err := json.Marshal(b.Payload)
	if err != nil {
		abortAPIError(c, 400, "VALIDATION_ERROR", "invalid payload")
		return
	}
	deviceTimeTrusted := false
	if deviceTime != nil && strings.TrimSpace(b.DeviceTimeSignature) != "" {
		deviceTimeTrusted = tracking.ValidateDeviceTimeHMAC(b.DeviceTime, b.DeviceTimeSignature, deviceKey)
	}

	processed := false
	if !late && b.SequenceNumber == lastApplied+1 {
		processed = true
	}
	var eventID string
	err = tx.QueryRow(c.Request.Context(), `
		INSERT INTO device_events(device_id, event_key, sequence_number, event_type, payload, received_at, device_time, device_time_trusted, late, processed)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id::text
	`, b.DeviceID, b.EventKey, b.SequenceNumber, b.EventType, string(payload), now, deviceTime, deviceTimeTrusted, late, processed).Scan(&eventID)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}

	if exceptionType, ok := exceptions.ExceptionTypeForEvent(b.EventType); ok {
		_, err = tx.Exec(c.Request.Context(), `
			INSERT INTO exceptions(device_id, exception_type, status, created_at)
			VALUES ($1, $2, 'open', $3)
		`, b.DeviceID, exceptionType, now)
		if err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
	}

	_, err = tx.Exec(c.Request.Context(), `
		UPDATE devices
		SET last_sequence_number = GREATEST(last_sequence_number, $2),
		    last_applied_sequence_number = CASE WHEN $4 THEN GREATEST(last_applied_sequence_number, $2) ELSE last_applied_sequence_number END,
		    last_event_received_at = $3,
		    status = 'online'
		WHERE id = $1
	`, b.DeviceID, b.SequenceNumber, now, processed)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}

	if !late {
		if err := h.applyBufferedEvents(c, tx, b.DeviceID); err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
	}

	if err := tx.Commit(c.Request.Context()); err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": eventID, "late": late, "reordered": reordered})
}

func (h *deviceHandler) applyBufferedEvents(c *gin.Context, tx pgx.Tx, deviceID string) error {
	for {
		var nextSeq int64
		err := tx.QueryRow(c.Request.Context(), `
			SELECT last_applied_sequence_number + 1
			FROM devices
			WHERE id = $1
			FOR UPDATE
		`, deviceID).Scan(&nextSeq)
		if err != nil {
			return err
		}

		var eventID string
		err = tx.QueryRow(c.Request.Context(), `
			SELECT id::text
			FROM device_events
			WHERE device_id = $1
			  AND sequence_number = $2
			  AND late = false
			  AND processed = false
			ORDER BY received_at ASC
			LIMIT 1
			FOR UPDATE
		`, deviceID, nextSeq).Scan(&eventID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}

		if _, err := tx.Exec(c.Request.Context(), `UPDATE device_events SET processed=true WHERE id=$1`, eventID); err != nil {
			return err
		}
		if _, err := tx.Exec(c.Request.Context(), `UPDATE devices SET last_applied_sequence_number=$2 WHERE id=$1`, deviceID, nextSeq); err != nil {
			return err
		}
	}
}

func (h *deviceHandler) replayDeviceEvents(c *gin.Context) {
	var b struct {
		EventKeys []string `json:"event_keys"`
	}
	if c.ShouldBindJSON(&b) != nil || len(b.EventKeys) == 0 {
		abortAPIError(c, 400, "VALIDATION_ERROR", "event_keys is required")
		return
	}

	now := time.Now().UTC()
	tx, err := h.pool.Begin(c.Request.Context())
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	defer tx.Rollback(c.Request.Context())

	replayed := 0
	skipped := 0
	for _, key := range b.EventKeys {
		key = strings.TrimSpace(key)
		if key == "" {
			skipped++
			continue
		}
		var id string
		var replayCount int
		err := tx.QueryRow(c.Request.Context(), `
			SELECT id::text, replay_count
			FROM device_events
			WHERE event_key=$1
			FOR UPDATE
		`, key).Scan(&id, &replayCount)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				skipped++
				continue
			}
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
		doReplay, doSkip := devices.ReplayDecision(replayCount)
		if doSkip {
			skipped++
			continue
		}
		if doReplay {
			_, err = tx.Exec(c.Request.Context(), `
				UPDATE device_events
				SET processed=true, replay_count=replay_count+1, replayed_at=$2
				WHERE id=$1
			`, id, now)
			if err != nil {
				abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
				return
			}
			replayed++
		}
	}

	actorID := getActorID(c)
	var actor *string
	if actorID != nil {
		actor = actorID
	}
	_ = h.authService.WriteAuditLog(c.Request.Context(), actor, "device_replay", "device_event", nil, map[string]any{"replayed": replayed, "skipped": skipped})

	if err := tx.Commit(c.Request.Context()); err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}

	c.JSON(http.StatusOK, gin.H{"replayed": replayed, "skipped": skipped})
}

func (h *deviceHandler) listDeviceEvents(c *gin.Context) {
	user, ok := getCurrentUser(c)
	if !ok {
		abortAPIError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	args := make([]any, 0)
	clauses := make([]string, 0)
	joins := ""
	if auth.HasAnyRole(user.Roles, []string{auth.RoleFleetManager}) {
		if user.OrganizationID == nil {
			abortAPIError(c, http.StatusForbidden, "FORBIDDEN", "forbidden")
			return
		}
		joins = " JOIN devices d ON d.id = device_events.device_id"
		args = append(args, *user.OrganizationID)
		clauses = append(clauses, "d.organization_id = $"+strconv.Itoa(len(args))+"::uuid")
	}
	if deviceID := strings.TrimSpace(c.Query("device_id")); deviceID != "" {
		args = append(args, deviceID)
		clauses = append(clauses, "device_id = $"+strconv.Itoa(len(args)))
	}
	if lateRaw := strings.TrimSpace(c.Query("late")); lateRaw != "" {
		late := strings.EqualFold(lateRaw, "true")
		args = append(args, late)
		clauses = append(clauses, "late = $"+strconv.Itoa(len(args)))
	}
	if fromRaw := strings.TrimSpace(c.Query("from")); fromRaw != "" {
		from, err := time.Parse(time.RFC3339, fromRaw)
		if err != nil {
			abortAPIError(c, 400, "VALIDATION_ERROR", "from must be RFC3339")
			return
		}
		args = append(args, from.UTC())
		clauses = append(clauses, "received_at >= $"+strconv.Itoa(len(args)))
	}
	if toRaw := strings.TrimSpace(c.Query("to")); toRaw != "" {
		to, err := time.Parse(time.RFC3339, toRaw)
		if err != nil {
			abortAPIError(c, 400, "VALIDATION_ERROR", "to must be RFC3339")
			return
		}
		args = append(args, to.UTC())
		clauses = append(clauses, "received_at <= $"+strconv.Itoa(len(args)))
	}
	page := parseIntDefault(c.Query("page"), 1)
	if page <= 0 {
		page = 1
	}
    limit := 50
    offset := (page - 1) * limit

	query := `
		SELECT id::text, device_id::text, event_key, sequence_number, event_type, COALESCE(payload, '{}'::jsonb), received_at, device_time, device_time_trusted, late, processed
		FROM device_events
	`
	if joins != "" {
		query += joins
	}
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	args = append(args, limit)
	limitPos := len(args)
	args = append(args, offset)
	offsetPos := len(args)
	query += " ORDER BY received_at DESC LIMIT $" + strconv.Itoa(limitPos) + " OFFSET $" + strconv.Itoa(offsetPos)

	rows, err := h.pool.Query(c.Request.Context(), query, args...)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	defer rows.Close()

	items := make([]gin.H, 0)
	for rows.Next() {
		var id, deviceID, eventKey, eventType string
		var seq int64
		var payload []byte
		var receivedAt time.Time
		var deviceTime *time.Time
		var trusted, late, processed bool
		if err := rows.Scan(&id, &deviceID, &eventKey, &seq, &eventType, &payload, &receivedAt, &deviceTime, &trusted, &late, &processed); err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
		payloadObj := map[string]any{}
		_ = json.Unmarshal(payload, &payloadObj)
		item := gin.H{
			"id":                  id,
			"device_id":           deviceID,
			"event_key":           eventKey,
			"sequence_number":     seq,
			"event_type":          eventType,
			"payload":             payloadObj,
			"received_at":         receivedAt.UTC().Format(timeRFC3339),
			"device_time_trusted": trusted,
			"late":                late,
			"processed":           processed,
		}
		if deviceTime != nil {
			item["device_time"] = deviceTime.UTC().Format(timeRFC3339)
		}
		items = append(items, item)
	}

	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items), "page": page})
}
