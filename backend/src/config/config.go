package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// AppConfig stores runtime settings for mini program and payment modules.
type AppConfig struct {
	Port       int    `yaml:"port"`
	SQLitePath string `yaml:"sqlite_path"`
	// CORSAllowedOrigins lists browser origins allowed to call the API directly.
	// The admin frontend goes through the Vite proxy (same origin) and does not need an entry.
	CORSAllowedOrigins []string `yaml:"cors_allowed_origins"`
	Admin              struct {
		// Token protects /api/* admin endpoints. Empty means a random one is generated at startup.
		Token string `yaml:"token"`
	} `yaml:"admin"`
	Auth struct {
		TokenSecret     string `yaml:"token_secret"`
		TokenTTLSeconds int64  `yaml:"token_ttl_seconds"`
	} `yaml:"auth"`
	WeChat struct {
		AppID             string `yaml:"app_id"`
		AppSecret         string `yaml:"app_secret"`
		MchID             string `yaml:"mch_id"`
		MchSerialNo       string `yaml:"mch_serial_no"`
		APIV3Key          string `yaml:"api_v3_key"`
		PrivateKeyPath    string `yaml:"private_key_path"`
		NotifyURL         string `yaml:"notify_url"`
		DefaultTemplateID string `yaml:"default_template_id"`
		MockMode          bool   `yaml:"mock_mode"`
	} `yaml:"wechat"`
	Reminder struct {
		Enabled           bool  `yaml:"enabled"`
		IntervalSeconds   int64 `yaml:"interval_seconds"`
		RetryDelaySeconds int64 `yaml:"retry_delay_seconds"`
	} `yaml:"reminder"`
}

// Load reads config from yaml and env variables.
func Load(path string) AppConfig {
	cfg := AppConfig{}
	cfg.Port = 8080
	cfg.SQLitePath = "./bin/rentadmin.db"
	cfg.Auth.TokenTTLSeconds = 72 * 3600
	cfg.WeChat.MockMode = true
	cfg.Reminder.Enabled = true
	cfg.Reminder.IntervalSeconds = 600
	cfg.Reminder.RetryDelaySeconds = 300

	data, err := os.ReadFile(path)
	if err == nil {
		if unmarshalErr := yaml.Unmarshal(data, &cfg); unmarshalErr != nil {
			log.Printf("failed to parse config file %s: %v", path, unmarshalErr)
		}
	} else {
		log.Printf("config file %s not found, use defaults and env", path)
	}

	if v := os.Getenv("PORT"); v != "" {
		if port, convErr := strconv.Atoi(v); convErr == nil {
			cfg.Port = port
		}
	}
	if v := os.Getenv("SQLITE_PATH"); v != "" {
		cfg.SQLitePath = v
	}
	if v := os.Getenv("CORS_ALLOWED_ORIGINS"); v != "" {
		cfg.CORSAllowedOrigins = strings.Split(v, ",")
	}
	if v := os.Getenv("ADMIN_TOKEN"); v != "" {
		cfg.Admin.Token = v
	}
	if v := os.Getenv("AUTH_TOKEN_SECRET"); v != "" {
		cfg.Auth.TokenSecret = v
	}
	if v := os.Getenv("AUTH_TOKEN_TTL_SECONDS"); v != "" {
		if n, convErr := strconv.ParseInt(v, 10, 64); convErr == nil {
			cfg.Auth.TokenTTLSeconds = n
		}
	}
	if v := os.Getenv("WECHAT_APP_ID"); v != "" {
		cfg.WeChat.AppID = v
	}
	if v := os.Getenv("WECHAT_APP_SECRET"); v != "" {
		cfg.WeChat.AppSecret = v
	}
	if v := os.Getenv("WECHAT_MCH_ID"); v != "" {
		cfg.WeChat.MchID = v
	}
	if v := os.Getenv("WECHAT_MCH_SERIAL_NO"); v != "" {
		cfg.WeChat.MchSerialNo = v
	}
	if v := os.Getenv("WECHAT_API_V3_KEY"); v != "" {
		cfg.WeChat.APIV3Key = v
	}
	if v := os.Getenv("WECHAT_PRIVATE_KEY_PATH"); v != "" {
		cfg.WeChat.PrivateKeyPath = v
	}
	if v := os.Getenv("WECHAT_NOTIFY_URL"); v != "" {
		cfg.WeChat.NotifyURL = v
	}
	if v := os.Getenv("WECHAT_DEFAULT_TEMPLATE_ID"); v != "" {
		cfg.WeChat.DefaultTemplateID = v
	}
	if v := os.Getenv("WECHAT_MOCK_MODE"); v != "" {
		if b, convErr := strconv.ParseBool(v); convErr == nil {
			cfg.WeChat.MockMode = b
		}
	}
	if v := os.Getenv("REMINDER_ENABLED"); v != "" {
		if b, convErr := strconv.ParseBool(v); convErr == nil {
			cfg.Reminder.Enabled = b
		}
	}
	if v := os.Getenv("REMINDER_INTERVAL_SECONDS"); v != "" {
		if n, convErr := strconv.ParseInt(v, 10, 64); convErr == nil {
			cfg.Reminder.IntervalSeconds = n
		}
	}
	if v := os.Getenv("REMINDER_RETRY_DELAY_SECONDS"); v != "" {
		if n, convErr := strconv.ParseInt(v, 10, 64); convErr == nil {
			cfg.Reminder.RetryDelaySeconds = n
		}
	}

	return cfg
}
