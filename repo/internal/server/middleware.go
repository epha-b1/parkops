package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func requestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		logger.Info("http request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}
}

func recoveryJSON() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, rec any) {
		if isAPIPath(c.Request.URL.Path) {
			abortAPIError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
			return
		}
		c.AbortWithStatus(http.StatusInternalServerError)
	})
}

func isAPIPath(path string) bool {
	return len(path) >= 4 && path[:4] == "/api"
}
