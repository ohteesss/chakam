package config

import (
	"strings"
	"testing"
)

func validProductionConfig() *Config {
	httpConfig := loadHTTPConfig()
	httpConfig.Addr = ":8080"
	return &Config{
		Env:   "production",
		DB:    DBConfig{URL: "postgres://user:pass@db/chakam?sslmode=require"},
		Redis: RedisConfig{Host: "redis", Port: "6379"},
		WhatsApp: WhatsAppConfig{
			VerifyToken:   "verify-token",
			AppSecret:     "app-secret",
			AccessToken:   "access-token",
			PhoneNumberID: "phone-id",
			BaseURL:       "https://graph.facebook.com/v25.0",
		},
		Gemini:   GeminiConfig{APIKey: "gemini-key"},
		HTTP:     httpConfig,
		Security: SecurityConfig{InternalAPIKey: strings.Repeat("a", 32)},
	}
}

func TestValidateProduction(t *testing.T) {
	cfg := validProductionConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid production config rejected: %v", err)
	}
}

func TestValidateProductionRequiresWebhookSecret(t *testing.T) {
	cfg := validProductionConfig()
	cfg.WhatsApp.AppSecret = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected missing WHATSAPP_APP_SECRET to fail validation")
	}
}

func TestValidateProductionRejectsInsecureDatabase(t *testing.T) {
	cfg := validProductionConfig()
	cfg.DB.URL = "postgres://user:pass@db/chakam?sslmode=disable"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected sslmode=disable to fail validation")
	}
}

func TestValidateProductionSecretsAreScopedPerProcess(t *testing.T) {
	apiConfig := validProductionConfig()
	apiConfig.WhatsApp.AccessToken = ""
	apiConfig.WhatsApp.PhoneNumberID = ""
	apiConfig.Gemini.APIKey = ""
	if err := apiConfig.ValidateAPI(); err != nil {
		t.Fatalf("API rejected without worker-only secrets: %v", err)
	}

	workerConfig := validProductionConfig()
	workerConfig.WhatsApp.VerifyToken = ""
	workerConfig.WhatsApp.AppSecret = ""
	workerConfig.Security.InternalAPIKey = ""
	if err := workerConfig.ValidateWorker(); err != nil {
		t.Fatalf("worker rejected without API-only secrets: %v", err)
	}
}
