package routes

import (
	"rentadmin/src/handlers"

	"github.com/gin-gonic/gin"
)

// SetupRoutes configures all API routes
func SetupRoutes(
	r *gin.Engine,
	tenantHandler *handlers.TenantHandler,
	excelHandler *handlers.ExcelHandler,
	mpAuthHandler *handlers.MPAuthHandler,
	wxPayHandler *handlers.WXPayHandler,
) {
	// Enable CORS
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
	})

	api := r.Group("/api")
	{
		api.GET("/tenants", tenantHandler.GetAllTenants)
		api.POST("/tenants", tenantHandler.CreateOrUpdateTenant)
		api.DELETE("/tenants", tenantHandler.ClearTenants)
		api.PUT("/tenants/:id", tenantHandler.UpdateTenant)
		api.DELETE("/tenants/:id", tenantHandler.DeleteTenantByID)
		api.GET("/tenants/room/:room_number", tenantHandler.GetTenantsByRoom)
		api.DELETE("/tenants/room/:room_number", tenantHandler.DeleteTenantByRoom)
		api.GET("/income-summary", tenantHandler.GetIncomeSummary)
		api.POST("/excel/import", excelHandler.ImportExcel)
	}

	if mpAuthHandler != nil && wxPayHandler != nil {
		mp := r.Group("/api/mp")
		{
			mp.POST("/login", mpAuthHandler.Login)
			mp.POST("/bind", mpAuthHandler.BindRoom)
			mp.GET("/bills", mpAuthHandler.GetBills)
			mp.POST("/subscribe/record", mpAuthHandler.RecordSubscribe)
			mp.POST("/pay/orders", wxPayHandler.CreateOrder)
			mp.POST("/pay/notify", wxPayHandler.Notify)
			mp.GET("/pay/orders/:out_trade_no", wxPayHandler.GetOrder)
			mp.POST("/pay/orders/:out_trade_no/sync", wxPayHandler.SyncOrder)
		}

		admin := r.Group("/api/admin")
		{
			admin.POST("/reminders/send", wxPayHandler.TriggerReminders)
		}
	}
}
