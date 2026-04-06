package server

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "parkops/docs"
	"parkops/internal/auth"
	"parkops/internal/exports"
	"parkops/internal/segments"
	"parkops/internal/web"
)

func NewRouter(logger *slog.Logger, pool *pgxpool.Pool, encryptionKey []byte, exportFileStore ...*exports.FileStore) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(recoveryJSON(), requestLogger(logger))

	// Initialize session signing and secure-cookie from config env vars
	secureCookie = os.Getenv("APP_ENV") == "production"
	if s := os.Getenv("SESSION_SECRET"); s != "" {
		sessionSecret = s
	}

	authStore := auth.NewPostgresStore(pool)
	authService := auth.NewService(authStore)
	segmentService := segments.NewService(pool)
	registerAuthRoutes(r, authService)
	registerMasterDataRoutes(r, authService, pool, encryptionKey)
	registerReservationRoutes(r, authService, pool)
	registerDeviceRoutes(r, authService, pool, encryptionKey)
	registerTrackingRoutes(r, authService, pool, encryptionKey)
	registerNotificationRoutes(r, authService, pool)
	registerCampaignRoutes(r, authService, pool)
	registerSegmentRoutes(r, authService, pool, segmentService)
	segmentAuth := exports.NewSegmentAuthorizer(pool, segmentService)
	var fs *exports.FileStore
	if len(exportFileStore) > 0 && exportFileStore[0] != nil {
		fs = exportFileStore[0]
	} else {
		// Fallback for tests: use temp dir
		var err error
		fs, err = exports.NewFileStore(os.TempDir() + "/parkops-exports")
		if err != nil {
			logger.Error("failed to create export file store", "error", err)
		}
	}
	registerAnalyticsRoutes(r, authService, pool, segmentAuth, fs)

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
			renderPage(c, web.CrudPage(toWebUser(c), web.CrudPageConfig{
				Title: "Facilities", Path: "/facilities", APIBase: "/api/facilities",
				CanCreate: true, CanEdit: true, CanDelete: true,
				Fields: []web.CrudField{
					{Key: "name", Label: "Name", Type: "text", Required: true, Placeholder: "Downtown Garage"},
					{Key: "address", Label: "Address", Type: "text", Placeholder: "100 Main St"},
				},
			}))
		})
		pages.GET("/lots", func(c *gin.Context) {
			renderPage(c, web.CrudPage(toWebUser(c), web.CrudPageConfig{
				Title: "Lots", Path: "/lots", APIBase: "/api/lots",
				CanCreate: true, CanEdit: true, CanDelete: true,
				Fields: []web.CrudField{
					{Key: "facility_id", Label: "Facility", Type: "uuid", Required: true, LookupAPI: "/api/facilities", LookupLabel: "name"},
					{Key: "name", Label: "Name", Type: "text", Required: true, Placeholder: "Level 1"},
				},
			}))
		})
		pages.GET("/zones", func(c *gin.Context) {
			renderPage(c, web.CrudPage(toWebUser(c), web.CrudPageConfig{
				Title: "Zones", Path: "/zones", APIBase: "/api/zones",
				CanCreate: true, CanEdit: true, CanDelete: true,
				Fields: []web.CrudField{
					{Key: "lot_id", Label: "Lot", Type: "uuid", Required: true, LookupAPI: "/api/lots", LookupLabel: "name"},
					{Key: "name", Label: "Name", Type: "text", Required: true, Placeholder: "Zone A"},
					{Key: "total_stalls", Label: "Total Stalls", Type: "number", Required: true, Placeholder: "50"},
					{Key: "hold_timeout_minutes", Label: "Hold Timeout (min)", Type: "number", Required: true, Default: "15"},
				},
			}))
		})
		pages.GET("/rate-plans", func(c *gin.Context) {
			renderPage(c, web.CrudPage(toWebUser(c), web.CrudPageConfig{
				Title: "Rate Plans", Path: "/rate-plans", APIBase: "/api/rate-plans",
				CanCreate: true, CanEdit: true, CanDelete: true,
				Fields: []web.CrudField{
					{Key: "zone_id", Label: "Zone", Type: "uuid", Required: true, LookupAPI: "/api/zones", LookupLabel: "name"},
					{Key: "name", Label: "Name", Type: "text", Required: true, Placeholder: "Standard Hourly"},
					{Key: "rate_cents", Label: "Rate (cents)", Type: "number", Required: true, Placeholder: "250"},
					{Key: "period", Label: "Period", Type: "select", Required: true, Options: []web.Option{
						{Value: "hourly", Label: "Hourly"},
						{Value: "daily", Label: "Daily"},
						{Value: "monthly", Label: "Monthly"},
					}},
				},
			}))
		})
		pages.GET("/members", func(c *gin.Context) {
			renderPage(c, web.CrudPage(toWebUser(c), web.CrudPageConfig{
				Title: "Members", Path: "/members", APIBase: "/api/members",
				CanCreate: true, CanEdit: true, CanDelete: true,
				Fields: []web.CrudField{
					{Key: "display_name", Label: "Display Name", Type: "text", Required: true, Placeholder: "Alice Parker"},
					{Key: "contact_notes", Label: "Contact Notes", Type: "textarea", Placeholder: "Optional notes"},
				},
			}))
		})
		pages.GET("/vehicles", func(c *gin.Context) {
			renderPage(c, web.CrudPage(toWebUser(c), web.CrudPageConfig{
				Title: "Vehicles", Path: "/vehicles", APIBase: "/api/vehicles",
				CanCreate: true, CanEdit: true, CanDelete: true,
				Fields: []web.CrudField{
					{Key: "plate_number", Label: "Plate Number", Type: "text", Required: true, Placeholder: "ABC-1234"},
					{Key: "make", Label: "Make", Type: "text", Placeholder: "Toyota"},
					{Key: "model", Label: "Model", Type: "text", Placeholder: "Camry"},
				},
			}))
		})
		pages.GET("/drivers", func(c *gin.Context) {
			renderPage(c, web.CrudPage(toWebUser(c), web.CrudPageConfig{
				Title: "Drivers", Path: "/drivers", APIBase: "/api/drivers",
				CanCreate: true, CanEdit: true, CanDelete: true,
				Fields: []web.CrudField{
					{Key: "member_id", Label: "Member", Type: "uuid", Required: true, LookupAPI: "/api/members", LookupLabel: "display_name"},
					{Key: "licence_number", Label: "Licence Number", Type: "text", Required: true, Placeholder: "DL-001-2025"},
				},
			}))
		})
		pages.GET("/notifications", func(c *gin.Context) {
			renderPage(c, web.NotificationsPage(toWebUser(c)))
		})
		pages.GET("/campaigns", func(c *gin.Context) {
			renderPage(c, web.CrudPage(toWebUser(c), web.CrudPageConfig{
				Title: "Campaigns", Path: "/campaigns", APIBase: "/api/campaigns",
				CanCreate: true, CanEdit: true, CanDelete: true,
				Fields: []web.CrudField{
					{Key: "title", Label: "Title", Type: "text", Required: true, Placeholder: "Morning Ops"},
					{Key: "description", Label: "Description", Type: "textarea", Placeholder: "Checklist description"},
					{Key: "target_role", Label: "Target Role", Type: "select", Options: []web.Option{
						{Value: "", Label: "All roles"},
						{Value: "facility_admin", Label: "Facility Admin"},
						{Value: "dispatch_operator", Label: "Dispatch Operator"},
						{Value: "fleet_manager", Label: "Fleet Manager"},
					}},
				},
			}))
		})
		pages.GET("/segments", func(c *gin.Context) {
			renderPage(c, web.CrudPage(toWebUser(c), web.CrudPageConfig{
				Title: "Segments", Path: "/segments", APIBase: "/api/segments",
				CanCreate: true, CanEdit: true, CanDelete: true,
				Fields: []web.CrudField{
					{Key: "name", Label: "Name", Type: "text", Required: true, Placeholder: "High Arrears"},
					{Key: "filter_expression", Label: "Filter Expression (JSON)", Type: "textarea", Required: true, Placeholder: `{"arrears_balance_cents": {"gt": 5000}}`},
					{Key: "schedule", Label: "Schedule", Type: "select", Required: true, Options: []web.Option{
						{Value: "manual", Label: "Manual"},
						{Value: "nightly", Label: "Nightly (02:00 UTC)"},
					}, Default: "manual"},
				},
			}))
		})
		pages.GET("/tasks", func(c *gin.Context) {
			renderPage(c, web.TasksPage(toWebUser(c)))
		})
		pages.GET("/notification-prefs", func(c *gin.Context) {
			renderPage(c, web.NotificationPrefsPage(toWebUser(c)))
		})
		pages.GET("/analytics", func(c *gin.Context) {
			renderPage(c, web.AnalyticsPage(toWebUser(c)))
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
	swaggerGroup := r.Group("/swagger")
	swaggerGroup.Use(requireSession(authService), enforceForcePasswordChange(), requireRoles(authService, auth.RoleFacilityAdmin))
	{
		swaggerGroup.GET("/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

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
