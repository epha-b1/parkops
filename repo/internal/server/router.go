package server

import (
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "parkops/docs"
	"parkops/internal/auth"
	"parkops/internal/web"
)

func NewRouter(logger *slog.Logger, pool *pgxpool.Pool, encryptionKey []byte) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(recoveryJSON(), requestLogger(logger))

	authStore := auth.NewPostgresStore(pool)
	authService := auth.NewService(authStore)
	registerAuthRoutes(r, authService)
	registerMasterDataRoutes(r, authService, pool, encryptionKey)
	registerReservationRoutes(r, authService, pool)

	r.GET("/login", gin.WrapH(templ.Handler(web.LoginPage())))
	pages := r.Group("")
	pages.Use(requireSession(authService), enforceForcePasswordChange(), requireRoles(authService, allSystemRoles()...))
	{
		pages.GET("/dashboard", func(c *gin.Context) {
			renderPage(c, web.DashboardPage(toWebUser(c)))
		})
		pages.GET("/reservations", func(c *gin.Context) {
			renderPage(c, web.ReservationsPage(toWebUser(c)))
		})
		pages.GET("/capacity", func(c *gin.Context) {
			renderPage(c, web.CapacityPage(toWebUser(c)))
		})
		pages.GET("/facilities", func(c *gin.Context) {
			renderPage(c, web.ListPage(toWebUser(c), "Facilities", "/facilities", "/api/facilities"))
		})
		pages.GET("/lots", func(c *gin.Context) {
			renderPage(c, web.ListPage(toWebUser(c), "Lots", "/lots", "/api/lots"))
		})
		pages.GET("/zones", func(c *gin.Context) {
			renderPage(c, web.ListPage(toWebUser(c), "Zones", "/zones", "/api/zones"))
		})
		pages.GET("/rate-plans", func(c *gin.Context) {
			renderPage(c, web.ListPage(toWebUser(c), "Rate Plans", "/rate-plans", "/api/rate-plans"))
		})
		pages.GET("/members", func(c *gin.Context) {
			renderPage(c, web.ListPage(toWebUser(c), "Members", "/members", "/api/members"))
		})
		pages.GET("/vehicles", func(c *gin.Context) {
			renderPage(c, web.ListPage(toWebUser(c), "Vehicles", "/vehicles", "/api/vehicles"))
		})
		pages.GET("/drivers", func(c *gin.Context) {
			renderPage(c, web.ListPage(toWebUser(c), "Drivers", "/drivers", "/api/drivers"))
		})
		pages.GET("/notifications", func(c *gin.Context) {
			renderPage(c, web.ListPage(toWebUser(c), "Notifications", "/notifications", "/api/notifications"))
		})
		pages.GET("/audit", func(c *gin.Context) {
			renderPage(c, web.ListPage(toWebUser(c), "Audit Log", "/audit", "/api/admin/audit-logs"))
		})
		pages.GET("/admin/users", func(c *gin.Context) {
			renderPage(c, web.ListPage(toWebUser(c), "Admin Users", "/admin/users", "/api/admin/users"))
		})
	}

	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	r.NoRoute(func(c *gin.Context) {
		if isAPIPath(c.Request.URL.Path) {
			abortAPIError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
			return
		}
		c.Status(http.StatusNotFound)
	})

	r.NoMethod(func(c *gin.Context) {
		if isAPIPath(c.Request.URL.Path) {
			abortAPIError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
			return
		}
		c.Status(http.StatusMethodNotAllowed)
	})

	return r
}

func renderPage(c *gin.Context, component templ.Component) {
	templ.Handler(component).ServeHTTP(c.Writer, c.Request)
}

func toWebUser(c *gin.Context) web.CurrentUser {
	user, ok := getCurrentUser(c)
	if !ok {
		return web.CurrentUser{}
	}
	return web.CurrentUser{
		DisplayName: user.DisplayName,
		Username:    user.Username,
		Roles:       user.Roles,
	}
}
