package main

import (
	"fmt"
	"log"
	"time"

	"rentadmin/src/config"
	"rentadmin/src/database"
	"rentadmin/src/handlers"
	"rentadmin/src/routes"
	"rentadmin/src/services"
	"rentadmin/src/utils"

	"github.com/gin-gonic/gin"
)

func main() {
	utils.SetupLogging()
	appConfig := config.Load("./conf/global.yaml")

	db := database.OpenDB("./bin/rentadmin.db")
	defer db.Close()

	database.InitSchema(db)
	database.MigrateSchema(db) // Apply migrations if needed

	r := gin.Default()

	// Create handler instances
	tenantHandler := &handlers.TenantHandler{DB: db}
	excelHandler := &handlers.ExcelHandler{DB: db}
	authService := services.NewAuthService(appConfig.Auth.TokenSecret, appConfig.Auth.TokenTTLSeconds)
	weChatService := services.NewWeChatPayService(services.WeChatServiceConfig{
		AppID:             appConfig.WeChat.AppID,
		AppSecret:         appConfig.WeChat.AppSecret,
		MchID:             appConfig.WeChat.MchID,
		MchSerialNo:       appConfig.WeChat.MchSerialNo,
		APIV3Key:          appConfig.WeChat.APIV3Key,
		PrivateKeyPath:    appConfig.WeChat.PrivateKeyPath,
		NotifyURL:         appConfig.WeChat.NotifyURL,
		DefaultTemplateID: appConfig.WeChat.DefaultTemplateID,
		MockMode:          appConfig.WeChat.MockMode,
	})
	tenantPaymentService := services.NewTenantPaymentService(db)
	reminderService := services.NewReminderService(db, weChatService, appConfig.WeChat.DefaultTemplateID, appConfig.Reminder.RetryDelaySeconds)
	mpAuthHandler := &handlers.MPAuthHandler{
		DB:       db,
		Auth:     authService,
		WeChat:   weChatService,
		Reminder: reminderService,
	}
	wxPayHandler := &handlers.WXPayHandler{
		DB:            db,
		Auth:          authService,
		WeChat:        weChatService,
		TenantPayment: tenantPaymentService,
		Reminder:      reminderService,
	}

	// Setup routes
	routes.SetupRoutes(r, tenantHandler, excelHandler, mpAuthHandler, wxPayHandler)

	if appConfig.Reminder.Enabled {
		go func() {
			ticker := time.NewTicker(time.Duration(appConfig.Reminder.IntervalSeconds) * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				sent, failed, err := reminderService.RunDueReminders(100)
				if err != nil {
					log.Printf("scheduled reminder run failed: %v", err)
					continue
				}
				if failed > 0 {
					_, _, _ = reminderService.RetryFailedReminders(100)
				}
				if sent > 0 || failed > 0 {
					log.Printf("scheduled reminder run finished, sent=%d failed=%d", sent, failed)
				}
			}
		}()
	}

	fmt.Printf("Server is running on port %d\n", appConfig.Port)
	log.Fatal(r.Run(fmt.Sprintf(":%d", appConfig.Port)))
}
