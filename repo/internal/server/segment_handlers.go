package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"parkops/internal/auth"
	"parkops/internal/segments"
)

type segmentHandler struct {
	pool           *pgxpool.Pool
	authService    *auth.Service
	segmentService *segments.Service
}

func registerSegmentRoutes(r *gin.Engine, authService *auth.Service, pool *pgxpool.Pool, segmentService *segments.Service) {
	h := &segmentHandler{pool: pool, authService: authService, segmentService: segmentService}

	read := r.Group("/api")
	read.Use(requireSession(authService), enforceForcePasswordChange(), requireRoles(authService, allSystemRoles()...))
	{
		read.GET("/tags", h.listTags)
		read.GET("/members/:id/tags", h.getMemberTags)
		read.GET("/segments", h.listSegments)
		read.GET("/segments/:id", h.getSegment)
		read.GET("/segments/:id/runs", h.listSegmentRuns)
	}

	write := r.Group("/api")
	write.Use(requireSession(authService), enforceForcePasswordChange(), requireRoles(authService, auth.RoleFacilityAdmin))
	{
		write.POST("/tags", h.createTag)
		write.DELETE("/tags/:id", h.deleteTag)
		write.POST("/members/:id/tags", h.addMemberTag)
		write.DELETE("/members/:id/tags/:tagId", h.removeMemberTag)
		write.POST("/tags/export", h.exportTags)
		write.POST("/tags/import", h.importTags)
		write.POST("/segments", h.createSegment)
		write.PATCH("/segments/:id", h.patchSegment)
		write.DELETE("/segments/:id", h.deleteSegment)
		write.POST("/segments/:id/preview", h.previewSegment)
		write.POST("/segments/:id/run", h.runSegment)
	}
}

// ── Tags ──

func (h *segmentHandler) listTags(c *gin.Context) {
	rows, err := h.pool.Query(c.Request.Context(), `SELECT id::text, name, created_at FROM tags ORDER BY name`)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	defer rows.Close()
	out := make([]gin.H, 0)
	for rows.Next() {
		var id, name string
		var createdAt time.Time
		if err := rows.Scan(&id, &name, &createdAt); err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
		out = append(out, gin.H{"id": id, "name": name, "created_at": createdAt.UTC().Format(timeRFC3339)})
	}
	c.JSON(http.StatusOK, out)
}

func (h *segmentHandler) createTag(c *gin.Context) {
	var b struct {
		Name string `json:"name"`
	}
	if c.ShouldBindJSON(&b) != nil || strings.TrimSpace(b.Name) == "" {
		abortAPIError(c, 400, "VALIDATION_ERROR", "name is required")
		return
	}
	actor, _ := getCurrentUser(c)
	var id string
	err := h.pool.QueryRow(c.Request.Context(), `
		INSERT INTO tags(name) VALUES ($1) RETURNING id::text
	`, strings.TrimSpace(b.Name)).Scan(&id)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			abortAPIError(c, 409, "CONFLICT", "tag name already exists")
			return
		}
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	_ = h.authService.WriteAuditLog(c.Request.Context(), &actor.ID, "tag_create", "tag", &id, map[string]any{"name": b.Name})
	c.JSON(http.StatusCreated, gin.H{"id": id, "name": strings.TrimSpace(b.Name)})
}

func (h *segmentHandler) deleteTag(c *gin.Context) {
	actor, _ := getCurrentUser(c)
	tagID := c.Param("id")
	_, err := h.pool.Exec(c.Request.Context(), `DELETE FROM tags WHERE id=$1::uuid`, tagID)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	_ = h.authService.WriteAuditLog(c.Request.Context(), &actor.ID, "tag_delete", "tag", &tagID, map[string]any{})
	c.Status(http.StatusNoContent)
}

func (h *segmentHandler) getMemberTags(c *gin.Context) {
	if !h.assertMemberScope(c, c.Param("id")) {
		return
	}

	rows, err := h.pool.Query(c.Request.Context(), `
		SELECT t.id::text, t.name
		FROM member_tags mt
		JOIN tags t ON t.id = mt.tag_id
		WHERE mt.member_id=$1::uuid
		ORDER BY t.name
	`, c.Param("id"))
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	defer rows.Close()
	out := make([]gin.H, 0)
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
		out = append(out, gin.H{"id": id, "name": name})
	}
	c.JSON(http.StatusOK, out)
}

func (h *segmentHandler) addMemberTag(c *gin.Context) {
	var b struct {
		TagID string `json:"tag_id"`
	}
	if c.ShouldBindJSON(&b) != nil || strings.TrimSpace(b.TagID) == "" {
		abortAPIError(c, 400, "VALIDATION_ERROR", "tag_id is required")
		return
	}
	if !isValidUUID(strings.TrimSpace(b.TagID)) {
		abortAPIError(c, 400, "VALIDATION_ERROR", "tag_id must be a valid UUID")
		return
	}
	if !isValidUUID(c.Param("id")) {
		abortAPIError(c, 400, "VALIDATION_ERROR", "member id must be a valid UUID")
		return
	}
	if !h.assertMemberScope(c, c.Param("id")) {
		return
	}
	actor, _ := getCurrentUser(c)
	memberID := c.Param("id")
	_, err := h.pool.Exec(c.Request.Context(), `
		INSERT INTO member_tags(member_id, tag_id, assigned_by)
		VALUES ($1::uuid, $2::uuid, $3::uuid)
		ON CONFLICT (member_id, tag_id) DO NOTHING
	`, memberID, strings.TrimSpace(b.TagID), actor.ID)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	_ = h.authService.WriteAuditLog(c.Request.Context(), &actor.ID, "member_tag_add", "member", &memberID, map[string]any{"tag_id": b.TagID})
	c.JSON(http.StatusOK, gin.H{"message": "tag added"})
}

func (h *segmentHandler) removeMemberTag(c *gin.Context) {
	if !h.assertMemberScope(c, c.Param("id")) {
		return
	}

	actor, _ := getCurrentUser(c)
	memberID := c.Param("id")
	tagID := c.Param("tagId")
	_, err := h.pool.Exec(c.Request.Context(), `
		DELETE FROM member_tags WHERE member_id=$1::uuid AND tag_id=$2::uuid
	`, memberID, tagID)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	_ = h.authService.WriteAuditLog(c.Request.Context(), &actor.ID, "member_tag_remove", "member", &memberID, map[string]any{"tag_id": tagID})
	c.JSON(http.StatusOK, gin.H{"message": "tag removed"})
}

func (h *segmentHandler) assertMemberScope(c *gin.Context, memberID string) bool {
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

	var orgID string
	err := h.pool.QueryRow(c.Request.Context(), `SELECT organization_id::text FROM members WHERE id=$1::uuid`, memberID).Scan(&orgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			abortAPIError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
		} else {
			abortAPIError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		}
		return false
	}
	if orgID != *actor.OrganizationID {
		abortAPIError(c, http.StatusForbidden, "FORBIDDEN", "forbidden")
		return false
	}
	return true
}

// ── Tag Export / Import ──

func (h *segmentHandler) exportTags(c *gin.Context) {
	var b struct {
		MemberIDs []string `json:"member_ids"`
	}
	_ = c.ShouldBindJSON(&b)

	actor, _ := getCurrentUser(c)

	var snapshot map[string][]string // member_id -> [tag_names]
	if len(b.MemberIDs) > 0 {
		snapshot = make(map[string][]string)
		for _, mid := range b.MemberIDs {
			rows, err := h.pool.Query(c.Request.Context(), `
				SELECT t.name FROM member_tags mt JOIN tags t ON t.id=mt.tag_id WHERE mt.member_id=$1::uuid ORDER BY t.name
			`, mid)
			if err != nil {
				abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
				return
			}
			var tagNames []string
			for rows.Next() {
				var n string
				if err := rows.Scan(&n); err != nil {
					rows.Close()
					abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
					return
				}
				tagNames = append(tagNames, n)
			}
			rows.Close()
			snapshot[mid] = tagNames
		}
	} else {
		rows, err := h.pool.Query(c.Request.Context(), `
			SELECT mt.member_id::text, t.name
			FROM member_tags mt JOIN tags t ON t.id=mt.tag_id
			ORDER BY mt.member_id, t.name
		`)
		if err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
		defer rows.Close()
		snapshot = make(map[string][]string)
		for rows.Next() {
			var mid, tname string
			if err := rows.Scan(&mid, &tname); err != nil {
				abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
				return
			}
			snapshot[mid] = append(snapshot[mid], tname)
		}
	}

	snapshotJSON, _ := json.Marshal(snapshot)
	var id string
	err := h.pool.QueryRow(c.Request.Context(), `
		INSERT INTO tag_versions(exported_by, snapshot)
		VALUES ($1::uuid, $2::jsonb)
		RETURNING id::text
	`, actor.ID, string(snapshotJSON)).Scan(&id)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	_ = h.authService.WriteAuditLog(c.Request.Context(), &actor.ID, "tag_export", "tag_version", &id, map[string]any{"member_count": len(snapshot)})
	c.JSON(http.StatusOK, gin.H{"id": id, "exported_at": time.Now().UTC().Format(timeRFC3339), "snapshot": snapshot})
}

func (h *segmentHandler) importTags(c *gin.Context) {
	var b struct {
		Snapshot map[string][]string `json:"snapshot"`
	}
	if c.ShouldBindJSON(&b) != nil || b.Snapshot == nil {
		abortAPIError(c, 400, "VALIDATION_ERROR", "snapshot is required")
		return
	}
	actor, _ := getCurrentUser(c)
	ctx := c.Request.Context()

	tx, err := h.pool.Begin(ctx)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	defer tx.Rollback(ctx)

	for memberID, tagNames := range b.Snapshot {
		_, err := tx.Exec(ctx, `DELETE FROM member_tags WHERE member_id=$1::uuid`, memberID)
		if err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
		for _, tagName := range tagNames {
			var tagID string
			err := tx.QueryRow(ctx, `SELECT id::text FROM tags WHERE name=$1`, tagName).Scan(&tagID)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					err = tx.QueryRow(ctx, `INSERT INTO tags(name) VALUES ($1) RETURNING id::text`, tagName).Scan(&tagID)
					if err != nil {
						abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
						return
					}
				} else {
					abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
					return
				}
			}
			_, err = tx.Exec(ctx, `
				INSERT INTO member_tags(member_id, tag_id, assigned_by)
				VALUES ($1::uuid, $2::uuid, $3::uuid)
				ON CONFLICT (member_id, tag_id) DO NOTHING
			`, memberID, tagID, actor.ID)
			if err != nil {
				abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
				return
			}
		}
	}

	snapshotJSON, _ := json.Marshal(b.Snapshot)
	var versionID string
	err = tx.QueryRow(ctx, `
		INSERT INTO tag_versions(imported_by, imported_at, snapshot)
		VALUES ($1::uuid, now(), $2::jsonb)
		RETURNING id::text
	`, actor.ID, string(snapshotJSON)).Scan(&versionID)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}

	if err := tx.Commit(ctx); err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}

	_ = h.authService.WriteAuditLog(ctx, &actor.ID, "tag_import", "tag_version", &versionID, map[string]any{"member_count": len(b.Snapshot)})
	c.JSON(http.StatusOK, gin.H{"message": "tags restored", "version_id": versionID})
}

// ── Segments ──

func (h *segmentHandler) listSegments(c *gin.Context) {
	rows, err := h.pool.Query(c.Request.Context(), `
		SELECT id::text, name, filter_expression, schedule, created_at, updated_at
		FROM segment_definitions ORDER BY created_at DESC
	`)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	defer rows.Close()
	out := make([]gin.H, 0)
	for rows.Next() {
		var id, name, schedule string
		var filterExpr json.RawMessage
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &name, &filterExpr, &schedule, &createdAt, &updatedAt); err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
		var fe any
		_ = json.Unmarshal(filterExpr, &fe)
		out = append(out, gin.H{
			"id": id, "name": name, "filter_expression": fe, "schedule": schedule,
			"created_at": createdAt.UTC().Format(timeRFC3339), "updated_at": updatedAt.UTC().Format(timeRFC3339),
		})
	}
	c.JSON(http.StatusOK, out)
}

func (h *segmentHandler) createSegment(c *gin.Context) {
	var b struct {
		Name             string          `json:"name"`
		FilterExpression json.RawMessage `json:"filter_expression"`
		Schedule         string          `json:"schedule"`
	}
	if c.ShouldBindJSON(&b) != nil || strings.TrimSpace(b.Name) == "" || len(b.FilterExpression) == 0 {
		abortAPIError(c, 400, "VALIDATION_ERROR", "name and filter_expression required")
		return
	}
	if b.Schedule == "" {
		b.Schedule = "manual"
	}
	if b.Schedule != "manual" && b.Schedule != "nightly" {
		abortAPIError(c, 400, "VALIDATION_ERROR", "schedule must be manual or nightly")
		return
	}
	actor, _ := getCurrentUser(c)
	var id string
	err := h.pool.QueryRow(c.Request.Context(), `
		INSERT INTO segment_definitions(name, filter_expression, schedule, created_by)
		VALUES ($1, $2::jsonb, $3, $4::uuid)
		RETURNING id::text
	`, strings.TrimSpace(b.Name), string(b.FilterExpression), b.Schedule, actor.ID).Scan(&id)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	var fe any
	_ = json.Unmarshal(b.FilterExpression, &fe)
	c.JSON(http.StatusCreated, gin.H{"id": id, "name": strings.TrimSpace(b.Name), "filter_expression": fe, "schedule": b.Schedule})
}

func (h *segmentHandler) getSegment(c *gin.Context) {
	var id, name, schedule string
	var filterExpr json.RawMessage
	var createdAt, updatedAt time.Time
	err := h.pool.QueryRow(c.Request.Context(), `
		SELECT id::text, name, filter_expression, schedule, created_at, updated_at
		FROM segment_definitions WHERE id=$1::uuid
	`, c.Param("id")).Scan(&id, &name, &filterExpr, &schedule, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			abortAPIError(c, 404, "NOT_FOUND", "segment not found")
			return
		}
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	var fe any
	_ = json.Unmarshal(filterExpr, &fe)
	c.JSON(http.StatusOK, gin.H{
		"id": id, "name": name, "filter_expression": fe, "schedule": schedule,
		"created_at": createdAt.UTC().Format(timeRFC3339), "updated_at": updatedAt.UTC().Format(timeRFC3339),
	})
}

func (h *segmentHandler) patchSegment(c *gin.Context) {
	var b struct {
		Name             *string          `json:"name"`
		FilterExpression *json.RawMessage `json:"filter_expression"`
		Schedule         *string          `json:"schedule"`
	}
	if c.ShouldBindJSON(&b) != nil {
		abortAPIError(c, 400, "VALIDATION_ERROR", "invalid request body")
		return
	}
	if b.Schedule != nil && *b.Schedule != "manual" && *b.Schedule != "nightly" {
		abortAPIError(c, 400, "VALIDATION_ERROR", "schedule must be manual or nightly")
		return
	}
	var feStr any
	if b.FilterExpression != nil {
		feStr = string(*b.FilterExpression)
	}
	_, err := h.pool.Exec(c.Request.Context(), `
		UPDATE segment_definitions
		SET name=COALESCE(NULLIF($2,''), name),
			filter_expression=COALESCE($3::jsonb, filter_expression),
			schedule=COALESCE($4, schedule),
			updated_at=now()
		WHERE id=$1::uuid
	`, c.Param("id"), valueOrEmpty(b.Name), feStr, b.Schedule)
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func (h *segmentHandler) deleteSegment(c *gin.Context) {
	_, err := h.pool.Exec(c.Request.Context(), `DELETE FROM segment_definitions WHERE id=$1::uuid`, c.Param("id"))
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *segmentHandler) previewSegment(c *gin.Context) {
	var filterExpr json.RawMessage
	err := h.pool.QueryRow(c.Request.Context(), `
		SELECT filter_expression FROM segment_definitions WHERE id=$1::uuid
	`, c.Param("id")).Scan(&filterExpr)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			abortAPIError(c, 404, "NOT_FOUND", "segment not found")
			return
		}
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	memberIDs, err := h.segmentService.EvaluateSegment(c.Request.Context(), filterExpr)
	if err != nil {
		abortAPIError(c, 400, "VALIDATION_ERROR", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"member_count": len(memberIDs)})
}

func (h *segmentHandler) runSegment(c *gin.Context) {
	actor, _ := getCurrentUser(c)
	segmentID := c.Param("id")
	runID, memberCount, err := h.segmentService.RunSegment(c.Request.Context(), segmentID, "manual")
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			abortAPIError(c, 404, "NOT_FOUND", "segment not found")
			return
		}
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	_ = h.authService.WriteAuditLog(c.Request.Context(), &actor.ID, "segment_run", "segment", &segmentID, map[string]any{"run_id": runID, "member_count": memberCount})
	c.JSON(http.StatusOK, gin.H{"id": runID, "segment_id": segmentID, "member_count": memberCount, "triggered_by": "manual", "ran_at": time.Now().UTC().Format(timeRFC3339)})
}

func (h *segmentHandler) listSegmentRuns(c *gin.Context) {
	rows, err := h.pool.Query(c.Request.Context(), `
		SELECT id::text, segment_id::text, ran_at, member_count, triggered_by
		FROM segment_runs WHERE segment_id=$1::uuid ORDER BY ran_at DESC
	`, c.Param("id"))
	if err != nil {
		abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
		return
	}
	defer rows.Close()
	out := make([]gin.H, 0)
	for rows.Next() {
		var id, segID, triggeredBy string
		var ranAt time.Time
		var memberCount int
		if err := rows.Scan(&id, &segID, &ranAt, &memberCount, &triggeredBy); err != nil {
			abortAPIError(c, 500, "INTERNAL_ERROR", "internal server error")
			return
		}
		out = append(out, gin.H{"id": id, "segment_id": segID, "ran_at": ranAt.UTC().Format(timeRFC3339), "member_count": memberCount, "triggered_by": triggeredBy})
	}
	c.JSON(http.StatusOK, out)
}
