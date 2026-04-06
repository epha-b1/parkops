package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"parkops/internal/auth"
)

const currentUserKey = "current_user"

func requireSession(authService *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		rawCookie, err := c.Cookie(auth.SessionCookieName)
		if err != nil || rawCookie == "" {
			if !isAPIPath(c.Request.URL.Path) {
				c.Redirect(http.StatusSeeOther, "/login?toast=session_expired")
				return
			}
			abortAPIError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
			return
		}

		sessionCookie, valid := verifySessionID(rawCookie)
		if !valid {
			clearSessionCookie(c)
			if !isAPIPath(c.Request.URL.Path) {
				c.Redirect(http.StatusSeeOther, "/login?toast=session_expired")
				return
			}
			abortAPIError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
			return
		}

		user, session, err := authService.AuthenticateSession(c.Request.Context(), sessionCookie)
		if err != nil {
			clearSessionCookie(c)
			if !isAPIPath(c.Request.URL.Path) {
				c.Redirect(http.StatusSeeOther, "/login?toast=session_expired")
				return
			}
			abortAPIError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
			return
		}

		c.Set(currentUserKey, user)
		setSessionCookie(c, session.ID)
		c.Next()
	}
}

func requireRoles(authService *auth.Service, allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := getCurrentUser(c)
		if !ok {
			abortAPIError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
			return
		}

		if !auth.HasAnyRole(user.Roles, allowedRoles) {
			actorID := user.ID
			_ = authService.WriteAuditLog(c.Request.Context(), &actorID, "rbac_denied", "route", nil, map[string]any{
				"method":         c.Request.Method,
				"path":           c.Request.URL.Path,
				"required_roles": allowedRoles,
				"user_roles":     user.Roles,
			})
			abortAPIError(c, http.StatusForbidden, "FORBIDDEN", "forbidden")
			return
		}

		c.Next()
	}
}

func enforceForcePasswordChange() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := getCurrentUser(c)
		if !ok {
			abortAPIError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
			return
		}

		if auth.ShouldForcePasswordChangeBlock(user.ForcePasswordChange, c.Request.Method, c.Request.URL.Path) {
			abortAPIError(c, http.StatusForbidden, "FORBIDDEN", "password change required")
			return
		}

		c.Next()
	}
}

func getCurrentUser(c *gin.Context) (auth.User, bool) {
	v, ok := c.Get(currentUserKey)
	if !ok {
		return auth.User{}, false
	}
	u, ok := v.(auth.User)
	if !ok {
		return auth.User{}, false
	}
	return u, true
}
