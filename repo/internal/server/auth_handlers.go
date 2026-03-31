package server

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"parkops/internal/auth"
)

type userResponse struct {
	ID                  string   `json:"id"`
	Username            string   `json:"username"`
	DisplayName         string   `json:"display_name"`
	Status              string   `json:"status"`
	Roles               []string `json:"roles"`
	ForcePasswordChange bool     `json:"force_password_change"`
	CreatedAt           string   `json:"created_at"`
}

type sessionResponse struct {
	ID           string `json:"id"`
	CreatedAt    string `json:"created_at"`
	LastActiveAt string `json:"last_active_at"`
	ExpiresAt    string `json:"expires_at"`
}

type pagedUsersResponse struct {
	Items []userResponse `json:"items"`
	Total int            `json:"total"`
	Page  int            `json:"page"`
	Limit int            `json:"limit"`
}

type auditLogResponse struct {
	ID           string         `json:"id"`
	ActorID      *string        `json:"actor_id"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type"`
	ResourceID   *string        `json:"resource_id"`
	Detail       map[string]any `json:"detail"`
	CreatedAt    string         `json:"created_at"`
}

func registerAuthRoutes(r *gin.Engine, authService *auth.Service) {
	r.POST("/api/auth/login", apiLoginHandler(authService))
	r.POST("/api/auth/logout", requireSession(authService), enforceForcePasswordChange(), requireRoles(authService, allSystemRoles()...), apiLogoutHandler(authService))
	r.POST("/auth/login", formLoginHandler(authService))
	r.POST("/auth/logout", requireSession(authService), enforceForcePasswordChange(), requireRoles(authService, allSystemRoles()...), formLogoutHandler(authService))

	authenticated := r.Group("/api")
	authenticated.Use(requireSession(authService), enforceForcePasswordChange(), requireRoles(authService, allSystemRoles()...))
	{
		authenticated.GET("/me", meHandler())
		authenticated.PATCH("/me/password", changeMyPasswordHandler(authService))
	}

	adminUsers := r.Group("/api/admin")
	adminUsers.Use(requireSession(authService), enforceForcePasswordChange(), requireRoles(authService, auth.RoleFacilityAdmin))
	{
		adminUsers.GET("/users", listUsersHandler(authService))
		adminUsers.POST("/users", createUserHandler(authService))
		adminUsers.PATCH("/users/:id", updateUserHandler(authService))
		adminUsers.DELETE("/users/:id", deleteUserHandler(authService))
		adminUsers.PATCH("/users/:id/roles", updateUserRolesHandler(authService))
		adminUsers.POST("/users/:id/unlock", unlockUserHandler(authService))
		adminUsers.GET("/users/:id/sessions", listUserSessionsHandler(authService))
		adminUsers.DELETE("/users/:id/sessions", deleteUserSessionsHandler(authService))
		adminUsers.POST("/users/:id/reset-password", adminResetPasswordHandler(authService))
	}

	auditViewer := r.Group("/api/admin")
	auditViewer.Use(requireSession(authService), enforceForcePasswordChange(), requireRoles(authService, auth.RoleAuditor))
	{
		auditViewer.GET("/audit-logs", listAuditLogsHandler(authService))
	}
}

func allSystemRoles() []string {
	return []string{auth.RoleFacilityAdmin, auth.RoleDispatch, auth.RoleFleetManager, auth.RoleAuditor}
}

func apiLoginHandler(authService *auth.Service) gin.HandlerFunc {
	type reqBody struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	return func(c *gin.Context) {
		var body reqBody
		if err := c.ShouldBindJSON(&body); err != nil {
			abortAPIError(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
			return
		}

		result, err := authService.Login(c.Request.Context(), strings.TrimSpace(body.Username), body.Password)
		if err != nil {
			handleLoginError(c, err)
			return
		}

		setSessionCookie(c, result.Session.ID)
		c.JSON(http.StatusOK, toUserResponse(result.User))
	}
}

func apiLogoutHandler(authService *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID, _ := c.Cookie(auth.SessionCookieName)
		_ = authService.Logout(c.Request.Context(), sessionID)
		clearSessionCookie(c)
		c.Status(http.StatusNoContent)
	}
}

func formLoginHandler(authService *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		username := strings.TrimSpace(c.PostForm("username"))
		password := c.PostForm("password")

		result, err := authService.Login(c.Request.Context(), username, password)
		if err != nil {
			c.Redirect(http.StatusSeeOther, "/login")
			return
		}

		setSessionCookie(c, result.Session.ID)
		c.Redirect(http.StatusSeeOther, "/dashboard")
	}
}

func formLogoutHandler(authService *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID, _ := c.Cookie(auth.SessionCookieName)
		_ = authService.Logout(c.Request.Context(), sessionID)
		clearSessionCookie(c)
		c.Redirect(http.StatusSeeOther, "/login")
	}
}

func meHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := getCurrentUser(c)
		if !ok {
			abortAPIError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
			return
		}

		c.JSON(http.StatusOK, toUserResponse(user))
	}
}

func changeMyPasswordHandler(authService *auth.Service) gin.HandlerFunc {
	type reqBody struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}

	return func(c *gin.Context) {
		user, ok := getCurrentUser(c)
		if !ok {
			abortAPIError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
			return
		}

		var body reqBody
		if err := c.ShouldBindJSON(&body); err != nil {
			abortAPIError(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
			return
		}

		err := authService.ChangeOwnPassword(c.Request.Context(), user.ID, body.CurrentPassword, body.NewPassword)
		if err != nil {
			switch {
			case errors.Is(err, auth.ErrValidation):
				abortAPIError(c, http.StatusBadRequest, "VALIDATION_ERROR", strings.TrimPrefix(err.Error(), auth.ErrValidation.Error()+": "))
			case errors.Is(err, auth.ErrPasswordIncorrect):
				abortAPIError(c, http.StatusUnauthorized, "UNAUTHORIZED", "current password is incorrect")
			default:
				abortAPIError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
			}
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "password changed"})
	}
}

func unlockUserHandler(authService *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := authService.UnlockUser(c.Request.Context(), c.Param("id")); err != nil {
			abortAPIError(c, http.StatusNotFound, "NOT_FOUND", "user not found")
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "account unlocked"})
	}
}

func listUserSessionsHandler(authService *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessions, err := authService.ListUserSessions(c.Request.Context(), c.Param("id"))
		if err != nil {
			abortAPIError(c, http.StatusNotFound, "NOT_FOUND", "user not found")
			return
		}

		items := make([]sessionResponse, 0, len(sessions))
		for _, session := range sessions {
			items = append(items, sessionResponse{
				ID:           session.ID,
				CreatedAt:    session.CreatedAt.UTC().Format(timeRFC3339),
				LastActiveAt: session.LastActiveAt.UTC().Format(timeRFC3339),
				ExpiresAt:    session.ExpiresAt.UTC().Format(timeRFC3339),
			})
		}

		c.JSON(http.StatusOK, items)
	}
}

func deleteUserSessionsHandler(authService *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := authService.DeleteUserSessions(c.Request.Context(), c.Param("id")); err != nil {
			abortAPIError(c, http.StatusNotFound, "NOT_FOUND", "user not found")
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func adminResetPasswordHandler(authService *auth.Service) gin.HandlerFunc {
	type reqBody struct {
		NewPassword string `json:"new_password"`
	}

	return func(c *gin.Context) {
		var body reqBody
		if err := c.ShouldBindJSON(&body); err != nil {
			abortAPIError(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
			return
		}

		err := authService.AdminResetPassword(c.Request.Context(), c.Param("id"), body.NewPassword)
		if err != nil {
			if errors.Is(err, auth.ErrValidation) {
				abortAPIError(c, http.StatusBadRequest, "VALIDATION_ERROR", strings.TrimPrefix(err.Error(), auth.ErrValidation.Error()+": "))
				return
			}
			abortAPIError(c, http.StatusNotFound, "NOT_FOUND", "user not found")
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "password reset, user must change on next login"})
	}
}

func listUsersHandler(authService *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, limit := parsePagination(c)
		users, total, err := authService.ListUsers(c.Request.Context(), page, limit)
		if err != nil {
			abortAPIError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
			return
		}

		items := make([]userResponse, 0, len(users))
		for _, u := range users {
			items = append(items, toUserResponse(u))
		}

		c.JSON(http.StatusOK, pagedUsersResponse{Items: items, Total: total, Page: page, Limit: limit})
	}
}

func createUserHandler(authService *auth.Service) gin.HandlerFunc {
	type reqBody struct {
		Username    string   `json:"username"`
		DisplayName string   `json:"display_name"`
		Password    string   `json:"password"`
		Roles       []string `json:"roles"`
	}

	return func(c *gin.Context) {
		var body reqBody
		if err := c.ShouldBindJSON(&body); err != nil {
			abortAPIError(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
			return
		}

		created, err := authService.CreateUser(c.Request.Context(), strings.TrimSpace(body.Username), strings.TrimSpace(body.DisplayName), body.Password, body.Roles)
		if err != nil {
			if errors.Is(err, auth.ErrValidation) {
				abortAPIError(c, http.StatusBadRequest, "VALIDATION_ERROR", strings.TrimPrefix(err.Error(), auth.ErrValidation.Error()+": "))
				return
			}
			abortAPIError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
			return
		}

		actorID := getActorID(c)
		createdID := created.ID
		_ = authService.WriteAuditLog(c.Request.Context(), actorID, "admin_user_create", "user", &createdID, map[string]any{"roles": created.Roles})
		c.JSON(http.StatusCreated, toUserResponse(created))
	}
}

func updateUserHandler(authService *auth.Service) gin.HandlerFunc {
	type reqBody struct {
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Status      string `json:"status"`
	}

	return func(c *gin.Context) {
		var body reqBody
		if err := c.ShouldBindJSON(&body); err != nil {
			abortAPIError(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
			return
		}

		updated, err := authService.UpdateUser(c.Request.Context(), c.Param("id"), strings.TrimSpace(body.Username), strings.TrimSpace(body.DisplayName), strings.TrimSpace(body.Status))
		if err != nil {
			switch {
			case errors.Is(err, auth.ErrValidation):
				abortAPIError(c, http.StatusBadRequest, "VALIDATION_ERROR", strings.TrimPrefix(err.Error(), auth.ErrValidation.Error()+": "))
			case errors.Is(err, auth.ErrNotFound):
				abortAPIError(c, http.StatusNotFound, "NOT_FOUND", "user not found")
			default:
				abortAPIError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
			}
			return
		}

		actorID := getActorID(c)
		updatedID := updated.ID
		_ = authService.WriteAuditLog(c.Request.Context(), actorID, "admin_user_update", "user", &updatedID, map[string]any{"status": updated.Status})
		c.JSON(http.StatusOK, toUserResponse(updated))
	}
}

func deleteUserHandler(authService *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.Param("id")
		if err := authService.DeleteUser(c.Request.Context(), userID); err != nil {
			abortAPIError(c, http.StatusNotFound, "NOT_FOUND", "user not found")
			return
		}
		actorID := getActorID(c)
		_ = authService.WriteAuditLog(c.Request.Context(), actorID, "admin_user_delete", "user", &userID, map[string]any{})
		c.Status(http.StatusNoContent)
	}
}

func updateUserRolesHandler(authService *auth.Service) gin.HandlerFunc {
	type reqBody struct {
		Roles []string `json:"roles"`
	}

	return func(c *gin.Context) {
		var body reqBody
		if err := c.ShouldBindJSON(&body); err != nil {
			abortAPIError(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
			return
		}

		userID := c.Param("id")
		if err := authService.UpdateUserRoles(c.Request.Context(), userID, body.Roles); err != nil {
			if errors.Is(err, auth.ErrValidation) {
				abortAPIError(c, http.StatusBadRequest, "VALIDATION_ERROR", strings.TrimPrefix(err.Error(), auth.ErrValidation.Error()+": "))
				return
			}
			abortAPIError(c, http.StatusNotFound, "NOT_FOUND", "user not found")
			return
		}

		updated, err := authService.GetUser(c.Request.Context(), userID)
		if err != nil {
			abortAPIError(c, http.StatusNotFound, "NOT_FOUND", "user not found")
			return
		}

		actorID := getActorID(c)
		_ = authService.WriteAuditLog(c.Request.Context(), actorID, "admin_user_roles_update", "user", &userID, map[string]any{"roles": body.Roles})
		c.JSON(http.StatusOK, toUserResponse(updated))
	}
}

func listAuditLogsHandler(authService *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, limit := parsePagination(c)
		logs, total, err := authService.ListAuditLogs(c.Request.Context(), page, limit)
		if err != nil {
			abortAPIError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
			return
		}

		items := make([]auditLogResponse, 0, len(logs))
		for _, l := range logs {
			items = append(items, auditLogResponse{
				ID:           l.ID,
				ActorID:      l.ActorID,
				Action:       l.Action,
				ResourceType: l.ResourceType,
				ResourceID:   l.ResourceID,
				Detail:       auth.DecodeAuditDetail(l.Detail),
				CreatedAt:    l.CreatedAt.UTC().Format(timeRFC3339),
			})
		}

		c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "limit": limit})
	}
}

func parsePagination(c *gin.Context) (int, int) {
	page := 1
	limit := 20
	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			page = v
		}
	}
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
		}
	}
	return page, limit
}

func getActorID(c *gin.Context) *string {
	user, ok := getCurrentUser(c)
	if !ok {
		return nil
	}
	id := user.ID
	return &id
}

func handleLoginError(c *gin.Context, err error) {
	var loginErr *auth.LoginError
	_ = errors.As(err, &loginErr)

	switch {
	case errors.Is(err, auth.ErrRateLimited):
		response := gin.H{
			"code":    "RATE_LIMITED",
			"message": "account is locked, try again later",
		}
		if loginErr != nil && loginErr.LockedUntil != nil {
			response["locked_until"] = loginErr.LockedUntil.UTC().Format(timeRFC3339)
		}
		c.AbortWithStatusJSON(http.StatusTooManyRequests, response)
	default:
		attemptsRemaining := 0
		if loginErr != nil {
			attemptsRemaining = loginErr.AttemptsRemaining
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"code":               "UNAUTHORIZED",
			"message":            "invalid credentials",
			"attempts_remaining": attemptsRemaining,
		})
	}
}

func toUserResponse(u auth.User) userResponse {
	return userResponse{
		ID:                  u.ID,
		Username:            u.Username,
		DisplayName:         u.DisplayName,
		Status:              u.Status,
		Roles:               u.Roles,
		ForcePasswordChange: u.ForcePasswordChange,
		CreatedAt:           u.CreatedAt.UTC().Format(timeRFC3339),
	}
}
