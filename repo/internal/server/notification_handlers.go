package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"parkops/internal/auth"
)

type notificationHandler struct {
	pool *pgxpool.Pool
}

func registerNotificationRoutes(r *gin.Engine, authService *auth.Service, pool *pgxpool.Pool) {
	h := &notificationHandler{pool: pool}

	api := r.Group("/api")
	api.Use(requireSession(authService), enforceForcePasswordChange(), requireRoles(authService, allSystemRoles()...))
	{
		api.GET("/notification-topics", h.listTopics)
		api.POST("/notification-topics/:id/subscribe", h.subscribeTopic)
		api.DELETE("/notification-topics/:id/subscribe", h.unsubscribeTopic)
		api.GET("/notification-settings", h.getSettings)
		api.PATCH("/notification-settings", h.patchSettings)
		api.GET("/notification-settings/dnd", h.getDND)
		api.PATCH("/notification-settings/dnd", h.patchDND)
		api.GET("/notifications", h.listNotifications)
		api.GET("/notifications/:id", h.getNotification)
		api.PATCH("/notifications/:id/read", h.markRead)
		api.POST("/notifications/:id/dismiss", h.dismissNotification)
		api.GET("/notifications/export-packages", h.listExportPackages)
		api.GET("/notifications/export-packages/:id/download", h.downloadExportPackage)
	}
}

func currentUserID(c *gin.Context) (string, bool) {
	u, ok := getCurrentUser(c)
	if !ok {
		abortAPIError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return "", false
	}
	return u.ID, true
}

func (h *notificationHandler) listTopics(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		return
	}
	rows, err := h.pool.Query(c.Request.Context(), `
		SELECT t.id::text, t.name,
		EXISTS(SELECT 1 FROM notification_subscriptions s WHERE s.user_id=$1::uuid AND s.topic_id=t.id) AS subscribed
		FROM notification_topics t
		ORDER BY t.name
	`, uid)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	defer rows.Close()
	out := make([]gin.H, 0)
	for rows.Next() {
		var id, name string
		var sub bool
		if err := rows.Scan(&id, &name, &sub); err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
		out = append(out, gin.H{"id": id, "name": name, "subscribed": sub})
	}
	c.JSON(http.StatusOK, out)
}

func (h *notificationHandler) subscribeTopic(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		return
	}
	_, err := h.pool.Exec(c.Request.Context(), `
		INSERT INTO notification_subscriptions(user_id, topic_id)
		VALUES ($1::uuid, $2::uuid)
		ON CONFLICT (user_id, topic_id) DO NOTHING
	`, uid, c.Param("id"))
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "subscribed"})
}

func (h *notificationHandler) unsubscribeTopic(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		return
	}
	_, err := h.pool.Exec(c.Request.Context(), `DELETE FROM notification_subscriptions WHERE user_id=$1::uuid AND topic_id=$2::uuid`, uid, c.Param("id"))
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "unsubscribed"})
}

func (h *notificationHandler) getSettings(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		return
	}
	var start, end time.Time
	var enabled bool
	err := h.pool.QueryRow(c.Request.Context(), `
		SELECT start_time::time, end_time::time, enabled
		FROM user_dnd_settings
		WHERE user_id=$1::uuid
	`, uid).Scan(&start, &end, &enabled)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		start = time.Date(2000, 1, 1, 22, 0, 0, 0, time.UTC)
		end = time.Date(2000, 1, 1, 6, 0, 0, 0, time.UTC)
		enabled = false
	}
	c.JSON(http.StatusOK, gin.H{"dnd": gin.H{"start_time": start.Format("15:04"), "end_time": end.Format("15:04"), "enabled": enabled}})
}

func (h *notificationHandler) patchSettings(c *gin.Context) { h.patchDND(c) }

func (h *notificationHandler) getDND(c *gin.Context) {
	h.getSettings(c)
}

func (h *notificationHandler) patchDND(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		return
	}
	var b struct {
		DND struct {
			StartTime string `json:"start_time"`
			EndTime   string `json:"end_time"`
			Enabled   bool   `json:"enabled"`
		} `json:"dnd"`
		StartTime string `json:"start_time"`
		EndTime   string `json:"end_time"`
		Enabled   *bool  `json:"enabled"`
	}
	if c.ShouldBindJSON(&b) != nil {
		abortAPIError(c, 400, "VALIDATION_ERROR", "invalid request body")
		return
	}
	startRaw := strings.TrimSpace(b.DND.StartTime)
	endRaw := strings.TrimSpace(b.DND.EndTime)
	enabled := b.DND.Enabled
	if startRaw == "" {
		startRaw = strings.TrimSpace(b.StartTime)
	}
	if endRaw == "" {
		endRaw = strings.TrimSpace(b.EndTime)
	}
	if b.Enabled != nil {
		enabled = *b.Enabled
	}
	if startRaw == "" || endRaw == "" {
		abortAPIError(c, 400, "VALIDATION_ERROR", "start_time and end_time are required")
		return
	}
	start, err := time.Parse("15:04", startRaw)
	if err != nil {
		abortAPIError(c, 400, "VALIDATION_ERROR", "start_time must be HH:MM")
		return
	}
	end, err := time.Parse("15:04", endRaw)
	if err != nil {
		abortAPIError(c, 400, "VALIDATION_ERROR", "end_time must be HH:MM")
		return
	}
	_, err = h.pool.Exec(c.Request.Context(), `
		INSERT INTO user_dnd_settings(user_id, start_time, end_time, enabled, updated_at)
		VALUES ($1::uuid, $2::time, $3::time, $4, now())
		ON CONFLICT (user_id)
		DO UPDATE SET start_time=EXCLUDED.start_time, end_time=EXCLUDED.end_time, enabled=EXCLUDED.enabled, updated_at=now()
	`, uid, start.Format("15:04:05"), end.Format("15:04:05"), enabled)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	c.JSON(http.StatusOK, gin.H{"start_time": start.Format("15:04"), "end_time": end.Format("15:04"), "enabled": enabled})
}

func (h *notificationHandler) listNotifications(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		return
	}
	page := parseIntDefault(c.Query("page"), 1)
	if page <= 0 {
		page = 1
	}
	limit := 20
	offset := (page - 1) * limit
	args := []any{uid}
	where := "WHERE user_id=$1::uuid"
	if r := strings.TrimSpace(c.Query("read")); r != "" {
		readVal := strings.EqualFold(r, "true")
		args = append(args, readVal)
		where += " AND read=$" + strconv.Itoa(len(args))
	}
	args = append(args, limit, offset)
	q := fmt.Sprintf(`
		SELECT id::text, title, body, read, dismissed, created_at
		FROM notifications
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, len(args)-1, len(args))
	rows, err := h.pool.Query(c.Request.Context(), q, args...)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id, title, body string
		var read, dismissed bool
		var createdAt time.Time
		if err := rows.Scan(&id, &title, &body, &read, &dismissed, &createdAt); err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
		items = append(items, gin.H{"id": id, "title": title, "body": body, "read": read, "dismissed": dismissed, "created_at": createdAt.UTC().Format(timeRFC3339)})
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items), "page": page})
}

func (h *notificationHandler) getNotification(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		return
	}
	var id, title, body string
	var read, dismissed bool
	var createdAt time.Time
	err := h.pool.QueryRow(c.Request.Context(), `
		SELECT id::text, title, body, read, dismissed, created_at
		FROM notifications
		WHERE id=$1::uuid AND user_id=$2::uuid
	`, c.Param("id"), uid).Scan(&id, &title, &body, &read, &dismissed, &createdAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			abortAPIError(c, 404, "NOT_FOUND", "resource not found")
			return
		}
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "title": title, "body": body, "read": read, "dismissed": dismissed, "created_at": createdAt.UTC().Format(timeRFC3339)})
}

func (h *notificationHandler) markRead(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		return
	}
	_, err := h.pool.Exec(c.Request.Context(), `UPDATE notifications SET read=true, read_at=now() WHERE id=$1::uuid AND user_id=$2::uuid`, c.Param("id"), uid)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "read"})
}

func (h *notificationHandler) dismissNotification(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		return
	}
	_, err := h.pool.Exec(c.Request.Context(), `UPDATE notifications SET dismissed=true, dismissed_at=now() WHERE id=$1::uuid AND user_id=$2::uuid`, c.Param("id"), uid)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "dismissed"})
}

func (h *notificationHandler) listExportPackages(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		return
	}
	rows, err := h.pool.Query(c.Request.Context(), `
		SELECT id::text, channel, user_id::text, created_at, downloaded_at
		FROM notification_jobs
		WHERE user_id=$1::uuid
		ORDER BY created_at DESC
		LIMIT 200
	`, uid)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id, channel, userID string
		var createdAt time.Time
		var downloadedAt *time.Time
		if err := rows.Scan(&id, &channel, &userID, &createdAt, &downloadedAt); err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
		item := gin.H{"id": id, "channel": channel, "recipient": userID, "created_at": createdAt.UTC().Format(timeRFC3339)}
		if downloadedAt != nil {
			item["downloaded_at"] = downloadedAt.UTC().Format(timeRFC3339)
		}
		items = append(items, item)
	}
	c.JSON(http.StatusOK, items)
}

func (h *notificationHandler) downloadExportPackage(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		return
	}
	var id, channel, userID, title, body, status string
	var payload []byte
	var createdAt time.Time
	var deliveredAt *time.Time
	err := h.pool.QueryRow(c.Request.Context(), `
		SELECT nj.id::text, nj.channel, nj.user_id::text, nj.payload, n.title, n.body, nj.status, nj.created_at, nj.delivered_at
		FROM notification_jobs nj
		JOIN notifications n ON n.id = nj.notification_id
		WHERE nj.id=$1::uuid AND nj.user_id=$2::uuid
	`, c.Param("id"), uid).Scan(&id, &channel, &userID, &payload, &title, &body, &status, &createdAt, &deliveredAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			abortAPIError(c, 404, "NOT_FOUND", "resource not found")
			return
		}
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	_, _ = h.pool.Exec(c.Request.Context(), `UPDATE notification_jobs SET downloaded_at=now(), updated_at=now() WHERE id=$1::uuid`, id)
	content := map[string]any{}
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &content)
	}
	resp := gin.H{
		"id":         id,
		"channel":    channel,
		"recipient":  userID,
		"status":     status,
		"title":      title,
		"body":       body,
		"payload":    content,
		"created_at": createdAt.UTC().Format(timeRFC3339),
	}
	if deliveredAt != nil {
		resp["delivered_at"] = deliveredAt.UTC().Format(timeRFC3339)
	}
	c.JSON(http.StatusOK, resp)
}
