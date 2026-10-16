package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
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

	ensureSecrets(&appConfig)

	db := database.OpenDB(appConfig.SQLitePath)
	defer db.Close()

	database.InitSchema(db)
	database.MigrateSchema(db) // Apply migrations if needed
	if n, err := services.BackfillDueMonths(db); err != nil {
		log.Fatalf("backfill due months: %v", err)
	} else if n > 0 {
		log.Printf("backfilled due_month for %d bills", n)
	}

	r := gin.Default()
	// Only the local Vite dev proxy may set X-Forwarded-For; everything else uses the socket address.
	if err := r.SetTrustedProxies([]string{"127.0.0.1", "::1"}); err != nil {
		log.Fatal(err)
	}

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
	routes.SetupRoutes(r, routes.RouteConfig{
		AdminToken:         appConfig.Admin.Token,
		CORSAllowedOrigins: appConfig.CORSAllowedOrigins,
		Auth:               authService,
		MockPayment:        appConfig.WeChat.MockMode,
	}, tenantHandler, excelHandler, mpAuthHandler, wxPayHandler)

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

// ensureSecrets fills in missing secrets. Outside mock mode a token secret is mandatory;
// in mock mode an ephemeral one is generated (mini program tokens then reset on restart).
func ensureSecrets(cfg *config.AppConfig) {
	if cfg.Auth.TokenSecret == "" {
		if !cfg.WeChat.MockMode {
			log.Fatal("auth.token_secret (or AUTH_TOKEN_SECRET) is required when wechat.mock_mode is false")
		}
		cfg.Auth.TokenSecret = randomHex(32)
		log.Printf("WARNING: AUTH_TOKEN_SECRET not set, using an ephemeral secret (mock mode only)")
	}
	if cfg.WeChat.MockMode {
		log.Printf("WARNING: wechat.mock_mode is ON — any login code is accepted and payments are simulated. Never expose this server publicly.")
	}
	if cfg.Admin.Token == "" {
		cfg.Admin.Token = randomHex(16)
		// Printed to the console only (not the log file); set ADMIN_TOKEN to keep it stable across restarts.
		fmt.Fprintf(os.Stderr, "\n  Admin token (enter it in the web console): %s\n\n", cfg.Admin.Token)
	}
}

func randomHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		log.Fatal(err)
	}
	return hex.EncodeToString(buf)
}
