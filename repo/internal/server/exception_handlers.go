package server

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"parkops/internal/exceptions"
)

func (h *reservationHandler) listOpenExceptions(c *gin.Context) {
	status := strings.TrimSpace(strings.ToLower(c.Query("status")))
	if status == "" {
		status = "open"
	}
	if status != "open" && status != "acknowledged" {
		abortAPIError(c, http.StatusBadRequest, "VALIDATION_ERROR", "status must be open|acknowledged")
		return
	}

	rows, err := h.pool.Query(c.Request.Context(), `
		SELECT id::text, device_id::text, exception_type, status, acknowledged_by::text, acknowledged_at, note, created_at
		FROM exceptions
		WHERE status = $1
		ORDER BY created_at DESC
	`, status)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	defer rows.Close()

	items := make([]gin.H, 0)
	for rows.Next() {
		var id, deviceID, exceptionType, rowStatus string
		var acknowledgedBy, note *string
		var acknowledgedAt *time.Time
		var createdAt time.Time
		if err := rows.Scan(&id, &deviceID, &exceptionType, &rowStatus, &acknowledgedBy, &acknowledgedAt, &note, &createdAt); err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
		item := gin.H{
			"id":             id,
			"device_id":      deviceID,
			"exception_type": exceptionType,
			"status":         rowStatus,
			"created_at":     createdAt.UTC().Format(timeRFC3339),
		}
		if acknowledgedBy != nil {
			item["acknowledged_by"] = *acknowledgedBy
		}
		if acknowledgedAt != nil {
			item["acknowledged_at"] = acknowledgedAt.UTC().Format(timeRFC3339)
		}
		if note != nil && strings.TrimSpace(*note) != "" {
			item["note"] = *note
		}
		items = append(items, item)
	}

	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

func (h *reservationHandler) getException(c *gin.Context) {
	var id, deviceID, exceptionType, status string
	var acknowledgedBy, note *string
	var acknowledgedAt *time.Time
	var createdAt time.Time
	err := h.pool.QueryRow(c.Request.Context(), `
		SELECT id::text, device_id::text, exception_type, status, acknowledged_by::text, acknowledged_at, note, created_at
		FROM exceptions
		WHERE id = $1
	`, c.Param("id")).Scan(&id, &deviceID, &exceptionType, &status, &acknowledgedBy, &acknowledgedAt, &note, &createdAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			abortAPIError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
			return
		}
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}

	item := gin.H{
		"id":             id,
		"device_id":      deviceID,
		"exception_type": exceptionType,
		"status":         status,
		"created_at":     createdAt.UTC().Format(timeRFC3339),
	}
	if acknowledgedBy != nil {
		item["acknowledged_by"] = *acknowledgedBy
	}
	if acknowledgedAt != nil {
		item["acknowledged_at"] = acknowledgedAt.UTC().Format(timeRFC3339)
	}
	if note != nil && strings.TrimSpace(*note) != "" {
		item["note"] = *note
	}
	c.JSON(http.StatusOK, item)
}

func (h *reservationHandler) acknowledgeException(c *gin.Context) {
	var b struct {
		Note string `json:"note"`
	}
	if c.Request.ContentLength > 0 && c.ShouldBindJSON(&b) != nil {
		abortAPIError(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}

	now := time.Now().UTC()
	actorID := getActorID(c)
	if actorID == nil || strings.TrimSpace(*actorID) == "" {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}

	tx, err := h.pool.Begin(c.Request.Context())
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	defer tx.Rollback(c.Request.Context())

	var currentStatus string
	err = tx.QueryRow(c.Request.Context(), `SELECT status FROM exceptions WHERE id=$1 FOR UPDATE`, c.Param("id")).Scan(&currentStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			abortAPIError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
			return
		}
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}

	if _, transitioned := exceptions.AcknowledgeTransition(currentStatus); transitioned {
		_, err = tx.Exec(c.Request.Context(), `
			UPDATE exceptions
			SET status='acknowledged', acknowledged_by=$2::uuid, acknowledged_at=$3, note=NULLIF($4, '')
			WHERE id=$1
		`, c.Param("id"), *actorID, now, strings.TrimSpace(b.Note))
		if err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
	}

	var id, deviceID, exceptionType, status string
	var acknowledgedBy, note *string
	var acknowledgedAt *time.Time
	var createdAt time.Time
	err = tx.QueryRow(c.Request.Context(), `
		SELECT id::text, device_id::text, exception_type, status, acknowledged_by::text, acknowledged_at, note, created_at
		FROM exceptions
		WHERE id = $1
	`, c.Param("id")).Scan(&id, &deviceID, &exceptionType, &status, &acknowledgedBy, &acknowledgedAt, &note, &createdAt)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}

	_ = h.authService.WriteAuditLog(c.Request.Context(), actorID, "exception_acknowledged", "exception", &id, map[string]any{"status": status})

	if err := tx.Commit(c.Request.Context()); err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}

	item := gin.H{
		"id":             id,
		"device_id":      deviceID,
		"exception_type": exceptionType,
		"status":         status,
		"created_at":     createdAt.UTC().Format(timeRFC3339),
	}
	if acknowledgedBy != nil {
		item["acknowledged_by"] = *acknowledgedBy
	}
	if acknowledgedAt != nil {
		item["acknowledged_at"] = acknowledgedAt.UTC().Format(timeRFC3339)
	}
	if note != nil && strings.TrimSpace(*note) != "" {
		item["note"] = *note
	}

	c.JSON(http.StatusOK, item)
}

func (h *reservationHandler) listExceptionHistory(c *gin.Context) {
	page := parseIntDefault(c.Query("page"), 1)
	if page <= 0 {
		page = 1
	}
	limit := 20
	offset := (page - 1) * limit

	rows, err := h.pool.Query(c.Request.Context(), `
		SELECT id::text, device_id::text, exception_type, status, acknowledged_by::text, acknowledged_at, note, created_at
		FROM exceptions
		WHERE status = 'acknowledged'
		ORDER BY acknowledged_at DESC, created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	defer rows.Close()

	items := make([]gin.H, 0)
	for rows.Next() {
		var id, deviceID, exceptionType, rowStatus string
		var acknowledgedBy, note *string
		var acknowledgedAt *time.Time
		var createdAt time.Time
		if err := rows.Scan(&id, &deviceID, &exceptionType, &rowStatus, &acknowledgedBy, &acknowledgedAt, &note, &createdAt); err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
		item := gin.H{
			"id":             id,
			"device_id":      deviceID,
			"exception_type": exceptionType,
			"status":         rowStatus,
			"created_at":     createdAt.UTC().Format(timeRFC3339),
		}
		if acknowledgedBy != nil {
			item["acknowledged_by"] = *acknowledgedBy
		}
		if acknowledgedAt != nil {
			item["acknowledged_at"] = acknowledgedAt.UTC().Format(timeRFC3339)
		}
		if note != nil && strings.TrimSpace(*note) != "" {
			item["note"] = *note
		}
		items = append(items, item)
	}

	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items), "page": page})
}
