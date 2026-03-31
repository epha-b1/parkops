package server

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"parkops/internal/auth"
)

type campaignHandler struct {
	pool        *pgxpool.Pool
	authService *auth.Service
}

func registerCampaignRoutes(r *gin.Engine, authService *auth.Service, pool *pgxpool.Pool) {
	h := &campaignHandler{pool: pool, authService: authService}

	read := r.Group("/api")
	read.Use(requireSession(authService), enforceForcePasswordChange(), requireRoles(authService, allSystemRoles()...))
	{
		read.GET("/campaigns", h.listCampaigns)
		read.GET("/campaigns/:id", h.getCampaign)
		read.GET("/campaigns/:id/tasks", h.listCampaignTasks)
		read.GET("/tasks/:id", h.getTask)
	}

	write := r.Group("/api")
	write.Use(requireSession(authService), enforceForcePasswordChange(), requireRoles(authService, auth.RoleFacilityAdmin))
	{
		write.POST("/campaigns", h.createCampaign)
		write.PATCH("/campaigns/:id", h.patchCampaign)
		write.DELETE("/campaigns/:id", h.deleteCampaign)
		write.POST("/campaigns/:id/tasks", h.createTask)
		write.PATCH("/tasks/:id", h.patchTask)
		write.DELETE("/tasks/:id", h.deleteTask)
		write.POST("/tasks/:id/complete", h.completeTask)
	}
}

func (h *campaignHandler) listCampaigns(c *gin.Context) {
	rows, err := h.pool.Query(c.Request.Context(), `
		SELECT id::text, title, description, COALESCE(target_role,''), created_by::text, created_at, updated_at
		FROM campaigns
		ORDER BY created_at DESC
	`)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	defer rows.Close()
	out := make([]gin.H, 0)
	for rows.Next() {
		var id, title, description, targetRole string
		var createdBy *string
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &title, &description, &targetRole, &createdBy, &createdAt, &updatedAt); err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
		item := gin.H{"id": id, "title": title, "description": description, "created_at": createdAt.UTC().Format(timeRFC3339), "updated_at": updatedAt.UTC().Format(timeRFC3339)}
		if targetRole != "" {
			item["target_role"] = targetRole
		}
		if createdBy != nil {
			item["created_by"] = *createdBy
		}
		out = append(out, item)
	}
	c.JSON(http.StatusOK, out)
}

func (h *campaignHandler) createCampaign(c *gin.Context) {
	var b struct {
		Title       string  `json:"title"`
		Description string  `json:"description"`
		TargetRole  *string `json:"target_role"`
	}
	if c.ShouldBindJSON(&b) != nil || strings.TrimSpace(b.Title) == "" {
		abortAPIError(c, 400, "VALIDATION_ERROR", "invalid request body")
		return
	}
	actor, _ := getCurrentUser(c)
	var id string
	err := h.pool.QueryRow(c.Request.Context(), `
		INSERT INTO campaigns(title, description, target_role, created_by)
		VALUES ($1, $2, NULLIF($3,''), $4::uuid)
		RETURNING id::text
	`, strings.TrimSpace(b.Title), strings.TrimSpace(b.Description), strings.TrimSpace(valueOrEmpty(b.TargetRole)), actor.ID).Scan(&id)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	_ = h.authService.WriteAuditLog(c.Request.Context(), &actor.ID, "campaign_create", "campaign", &id, map[string]any{"title": b.Title})
	c.JSON(http.StatusCreated, gin.H{"id": id, "title": strings.TrimSpace(b.Title), "description": strings.TrimSpace(b.Description), "target_role": strings.TrimSpace(valueOrEmpty(b.TargetRole))})
}

func (h *campaignHandler) getCampaign(c *gin.Context) {
	var id, title, description, targetRole string
	var createdBy *string
	var createdAt, updatedAt time.Time
	err := h.pool.QueryRow(c.Request.Context(), `
		SELECT id::text, title, description, COALESCE(target_role,''), created_by::text, created_at, updated_at
		FROM campaigns WHERE id=$1::uuid
	`, c.Param("id")).Scan(&id, &title, &description, &targetRole, &createdBy, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			abortAPIError(c, 404, "NOT_FOUND", "resource not found")
			return
		}
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	out := gin.H{"id": id, "title": title, "description": description, "created_at": createdAt.UTC().Format(timeRFC3339), "updated_at": updatedAt.UTC().Format(timeRFC3339)}
	if targetRole != "" {
		out["target_role"] = targetRole
	}
	if createdBy != nil {
		out["created_by"] = *createdBy
	}
	c.JSON(http.StatusOK, out)
}

func (h *campaignHandler) patchCampaign(c *gin.Context) {
	var b struct {
		Title       *string `json:"title"`
		Description *string `json:"description"`
		TargetRole  *string `json:"target_role"`
	}
	if c.ShouldBindJSON(&b) != nil {
		abortAPIError(c, 400, "VALIDATION_ERROR", "invalid request body")
		return
	}
	_, err := h.pool.Exec(c.Request.Context(), `
		UPDATE campaigns
		SET title=COALESCE(NULLIF($2,''), title),
			description=COALESCE($3, description),
			target_role=COALESCE($4, target_role),
			updated_at=now()
		WHERE id=$1::uuid
	`, c.Param("id"), strings.TrimSpace(valueOrEmpty(b.Title)), b.Description, b.TargetRole)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func (h *campaignHandler) deleteCampaign(c *gin.Context) {
	_, err := h.pool.Exec(c.Request.Context(), `DELETE FROM campaigns WHERE id=$1::uuid`, c.Param("id"))
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *campaignHandler) listCampaignTasks(c *gin.Context) {
	rows, err := h.pool.Query(c.Request.Context(), `
		SELECT id::text, campaign_id::text, description, deadline, reminder_interval_minutes, completed_at, last_reminder_at, created_at, updated_at
		FROM tasks
		WHERE campaign_id=$1::uuid
		ORDER BY created_at DESC
	`, c.Param("id"))
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		items = append(items, mustTaskRow(c, rows))
	}
	c.JSON(http.StatusOK, items)
}

func (h *campaignHandler) createTask(c *gin.Context) {
	var b struct {
		Description             string `json:"description"`
		Deadline                string `json:"deadline"`
		ReminderIntervalMinutes int    `json:"reminder_interval_minutes"`
	}
	if c.ShouldBindJSON(&b) != nil || strings.TrimSpace(b.Description) == "" {
		abortAPIError(c, 400, "VALIDATION_ERROR", "invalid request body")
		return
	}
	interval := b.ReminderIntervalMinutes
	if interval <= 0 {
		interval = 60
	}
	var deadline any
	if strings.TrimSpace(b.Deadline) != "" {
		d, err := time.Parse(time.RFC3339, b.Deadline)
		if err != nil {
			abortAPIError(c, 400, "VALIDATION_ERROR", "deadline must be RFC3339")
			return
		}
		deadline = d.UTC()
	}
	actor, _ := getCurrentUser(c)
	var id string
	err := h.pool.QueryRow(c.Request.Context(), `
		INSERT INTO tasks(campaign_id, description, deadline, reminder_interval_minutes, created_by)
		VALUES ($1::uuid, $2, $3, $4, $5::uuid)
		RETURNING id::text
	`, c.Param("id"), strings.TrimSpace(b.Description), deadline, interval, actor.ID).Scan(&id)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	_ = h.authService.WriteAuditLog(c.Request.Context(), &actor.ID, "task_create", "task", &id, map[string]any{"campaign_id": c.Param("id")})
	c.JSON(http.StatusCreated, gin.H{"id": id, "campaign_id": c.Param("id"), "description": strings.TrimSpace(b.Description), "reminder_interval_minutes": interval})
}

func (h *campaignHandler) getTask(c *gin.Context) {
	row := h.pool.QueryRow(c.Request.Context(), `
		SELECT id::text, campaign_id::text, description, deadline, reminder_interval_minutes, completed_at, last_reminder_at, created_at, updated_at
		FROM tasks
		WHERE id=$1::uuid
	`, c.Param("id"))
	var id, campaignID, description string
	var deadline, completedAt, lastReminderAt *time.Time
	var reminderInterval int
	var createdAt, updatedAt time.Time
	err := row.Scan(&id, &campaignID, &description, &deadline, &reminderInterval, &completedAt, &lastReminderAt, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			abortAPIError(c, 404, "NOT_FOUND", "resource not found")
			return
		}
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	out := taskJSON(id, campaignID, description, deadline, reminderInterval, completedAt, lastReminderAt, createdAt, updatedAt)
	c.JSON(http.StatusOK, out)
}

func (h *campaignHandler) patchTask(c *gin.Context) {
	var b struct {
		Description             *string `json:"description"`
		Deadline                *string `json:"deadline"`
		ReminderIntervalMinutes *int    `json:"reminder_interval_minutes"`
	}
	if c.ShouldBindJSON(&b) != nil {
		abortAPIError(c, 400, "VALIDATION_ERROR", "invalid request body")
		return
	}
	var deadline any
	if b.Deadline != nil {
		if strings.TrimSpace(*b.Deadline) == "" {
			deadline = nil
		} else {
			d, err := time.Parse(time.RFC3339, strings.TrimSpace(*b.Deadline))
			if err != nil {
				abortAPIError(c, 400, "VALIDATION_ERROR", "deadline must be RFC3339")
				return
			}
			deadline = d.UTC()
		}
	}
	var interval any
	if b.ReminderIntervalMinutes != nil {
		if *b.ReminderIntervalMinutes <= 0 {
			abortAPIError(c, 400, "VALIDATION_ERROR", "reminder_interval_minutes must be positive")
			return
		}
		interval = *b.ReminderIntervalMinutes
	}
	_, err := h.pool.Exec(c.Request.Context(), `
		UPDATE tasks
		SET description=COALESCE(NULLIF($2,''), description),
			deadline=COALESCE($3, deadline),
			reminder_interval_minutes=COALESCE($4, reminder_interval_minutes),
			updated_at=now()
		WHERE id=$1::uuid
	`, c.Param("id"), valueOrNilString(b.Description), deadline, interval)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func (h *campaignHandler) deleteTask(c *gin.Context) {
	_, err := h.pool.Exec(c.Request.Context(), `DELETE FROM tasks WHERE id=$1::uuid`, c.Param("id"))
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *campaignHandler) completeTask(c *gin.Context) {
	actor, _ := getCurrentUser(c)
	taskID := c.Param("id")
	_, err := h.pool.Exec(c.Request.Context(), `
		UPDATE tasks
		SET completed_at=now(), completed_by=$2::uuid, updated_at=now()
		WHERE id=$1::uuid
	`, taskID, actor.ID)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	_ = h.authService.WriteAuditLog(c.Request.Context(), &actor.ID, "task_complete", "task", &taskID, map[string]any{})
	h.getTask(c)
}

func valueOrEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(*v)
}

func valueOrNilString(v *string) any {
	if v == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return nil
	}
	return trimmed
}

func mustTaskRow(c *gin.Context, rows pgx.Rows) gin.H {
	var id, campaignID, description string
	var deadline, completedAt, lastReminderAt *time.Time
	var reminderInterval int
	var createdAt, updatedAt time.Time
	if err := rows.Scan(&id, &campaignID, &description, &deadline, &reminderInterval, &completedAt, &lastReminderAt, &createdAt, &updatedAt); err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return gin.H{}
	}
	return taskJSON(id, campaignID, description, deadline, reminderInterval, completedAt, lastReminderAt, createdAt, updatedAt)
}

func taskJSON(id, campaignID, description string, deadline *time.Time, reminderInterval int, completedAt, lastReminderAt *time.Time, createdAt, updatedAt time.Time) gin.H {
	out := gin.H{
		"id":                        id,
		"campaign_id":               campaignID,
		"description":               description,
		"reminder_interval_minutes": reminderInterval,
		"created_at":                createdAt.UTC().Format(timeRFC3339),
		"updated_at":                updatedAt.UTC().Format(timeRFC3339),
	}
	if deadline != nil {
		out["deadline"] = deadline.UTC().Format(timeRFC3339)
	}
	if completedAt != nil {
		out["completed_at"] = completedAt.UTC().Format(timeRFC3339)
	}
	if lastReminderAt != nil {
		out["last_reminder_at"] = lastReminderAt.UTC().Format(timeRFC3339)
	}
	return out
}
