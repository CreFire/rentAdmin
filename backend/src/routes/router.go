package routes

import (
	"rentadmin/src/handlers"
	"rentadmin/src/middleware"
	"rentadmin/src/services"

	"github.com/gin-gonic/gin"
)

// RouteConfig carries the security settings routes depend on.
type RouteConfig struct {
	AdminToken         string
	CORSAllowedOrigins []string
	Auth               *services.AuthService
	MockPayment        bool
}

// SetupRoutes configures all API routes
func SetupRoutes(
	r *gin.Engine,
	cfg RouteConfig,
	tenantHandler *handlers.TenantHandler,
	excelHandler *handlers.ExcelHandler,
	mpAuthHandler *handlers.MPAuthHandler,
	wxPayHandler *handlers.WXPayHandler,
) {
	r.Use(middleware.CORS(cfg.CORSAllowedOrigins))
	adminAuth := middleware.AdminAuth(cfg.AdminToken)

	api := r.Group("/api", adminAuth)
	{
		api.GET("/tenants", tenantHandler.GetAllTenants)
		api.POST("/tenants", tenantHandler.CreateOrUpdateTenant)
		api.DELETE("/tenants", tenantHandler.ClearTenants)
		api.PUT("/tenants/:id", tenantHandler.UpdateTenant)
		api.DELETE("/tenants/:id", tenantHandler.DeleteTenantByID)
		api.POST("/tenants/:id/payments", tenantHandler.RegisterPayment)
		api.GET("/tenants/room/:room_number", tenantHandler.GetTenantsByRoom)
		api.DELETE("/tenants/room/:room_number", tenantHandler.DeleteTenantByRoom)
		api.POST("/rooms/:room_number/payments", tenantHandler.RegisterRoomPayment)
		api.POST("/bills/generate", tenantHandler.GenerateMonthlyBills)
		api.GET("/income-summary", tenantHandler.GetIncomeSummary)
		api.POST("/excel/import", excelHandler.ImportExcel)
		api.POST("/admin/reminders/send", wxPayHandler.TriggerReminders)
	}

	// Mini program routes live outside the admin group: they use per-user tokens instead.
	r.POST("/api/mp/login", mpAuthHandler.Login)
	if cfg.MockPayment {
		// Local development only: the mini program reports a simulated payment for its own order.
		r.POST("/api/mp/pay/notify", middleware.MPAuth(cfg.Auth), wxPayHandler.MockNotify)
	} else {
		r.POST("/api/mp/pay/notify", wxPayHandler.Notify)
	}

	mp := r.Group("/api/mp", middleware.MPAuth(cfg.Auth))
	{
		mp.POST("/bind", mpAuthHandler.BindRoom)
		mp.GET("/bills", mpAuthHandler.GetBills)
		mp.POST("/subscribe/record", mpAuthHandler.RecordSubscribe)
		mp.POST("/pay/orders", wxPayHandler.CreateOrder)
		mp.GET("/pay/orders/:out_trade_no", wxPayHandler.GetOrder)
		mp.POST("/pay/orders/:out_trade_no/sync", wxPayHandler.SyncOrder)
	}
}
