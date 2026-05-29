package config

import (
	"log"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// AppConfig stores runtime settings for mini program and payment modules.
type AppConfig struct {
	Port int `yaml:"port"`
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
	cfg.Auth.TokenSecret = "change_me_in_production"
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
