package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config is the whole application's configuration in one value. main reads it
// once at startup and hands the pieces to whoever needs them.
type Config struct {
	DB       DBConfig
	WhatsApp WhatsAppConfig
	Redis    RedisConfig
	Gemini   GeminiConfig
	HTTP     HTTPConfig
	Security SecurityConfig
	Env      string // "development" | "production"
}

type DBConfig struct {
	URL      string
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type WhatsAppConfig struct {
	VerifyToken   string
	AppSecret     string
	AccessToken   string
	PhoneNumberID string
	BaseURL       string
}

type RedisConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	DB       int
	TLS      bool
}

type GeminiConfig struct {
	Model  string
	APIKey string
}

type HTTPConfig struct {
	Addr              string
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
	MaxBodyBytes      int64
	MaxHeaderBytes    int
}

type SecurityConfig struct {
	InternalAPIKey string
}

// IsDev reports whether we're running in development. Centralizing the check
// here means callers ask a clear question instead of string-comparing "Env"
// everywhere (and getting the comparison subtly wrong).
func (c *Config) IsDev() bool {
	return c.Env == "development"
}

// Load is the single entry point. It (1) pulls .env into the process
// environment, then (2) reads everything into a Config. It's an EXPLICIT
// function call — deliberately not an init() — so main controls when it runs
// and tests don't load .env as an import side effect.
func Load() *Config {
	// Side effect: load .env into the environment. A missing file is NOT an
	// error — in production you set real env vars and ship no .env — so we
	// intentionally ignore the result. godotenv does not override variables
	// already present, so real env / CI values always win over a local .env.
	_ = godotenv.Load()

	return &Config{
		DB:       loadDBConfig(),
		WhatsApp: loadWhatsAppConfig(),
		Redis:    loadRedisConfig(),
		Gemini:   loadGeminiConfig(),
		HTTP:     loadHTTPConfig(),
		Security: SecurityConfig{InternalAPIKey: getEnv("INTERNAL_API_KEY", "")},
		Env:      strings.ToLower(getEnv("ENV", "development")),
	}
}

// Validate checks the union of API and worker requirements. The binaries use
// the narrower methods below so each deployment receives only the secrets it
// needs.
func (c *Config) Validate() error {
	if err := c.ValidateAPI(); err != nil {
		return err
	}
	return c.ValidateWorker()
}

func (c *Config) ValidateAPI() error {
	if err := c.validateCommon(); err != nil {
		return err
	}
	if c.Env != "production" {
		return nil
	}
	required := map[string]string{
		"WHATSAPP_VERIFY_TOKEN": c.WhatsApp.VerifyToken,
		"WHATSAPP_APP_SECRET":   c.WhatsApp.AppSecret,
		"INTERNAL_API_KEY":      c.Security.InternalAPIKey,
	}
	if err := validateRequired(required); err != nil {
		return err
	}
	if len(c.Security.InternalAPIKey) < 32 {
		return errors.New("INTERNAL_API_KEY must contain at least 32 characters in production")
	}
	return nil
}

func (c *Config) ValidateWorker() error {
	if err := c.validateCommon(); err != nil {
		return err
	}
	if c.Env != "production" {
		return nil
	}
	required := map[string]string{
		"WHATSAPP_ACCESS_TOKEN":    c.WhatsApp.AccessToken,
		"WHATSAPP_PHONE_NUMBER_ID": c.WhatsApp.PhoneNumberID,
		"GEMINI_API_KEY":           c.Gemini.APIKey,
	}
	if err := validateRequired(required); err != nil {
		return err
	}
	baseURL, err := url.Parse(c.WhatsApp.BaseURL)
	if err != nil || baseURL.Scheme != "https" || baseURL.Host == "" {
		return errors.New("WHATSAPP_BASE_URL must be a valid HTTPS URL in production")
	}
	return nil
}

func (c *Config) validateCommon() error {
	if c.Env != "development" && c.Env != "production" && c.Env != "test" {
		return fmt.Errorf("ENV must be development, test, or production, got %q", c.Env)
	}
	if c.HTTP.Addr == "" {
		return errors.New("SERVER_ADDR must not be empty")
	}
	if c.DB.URL == "" && c.DB.Host == "" {
		return errors.New("DATABASE_URL or DB_HOST is required")
	}
	if c.Redis.Host == "" || c.Redis.Port == "" {
		return errors.New("REDIS_HOST and REDIS_PORT are required")
	}
	if c.HTTP.ReadTimeout <= 0 || c.HTTP.ReadHeaderTimeout <= 0 || c.HTTP.WriteTimeout <= 0 || c.HTTP.IdleTimeout <= 0 || c.HTTP.ShutdownTimeout <= 0 {
		return errors.New("HTTP timeout values must be positive")
	}
	if c.HTTP.MaxBodyBytes <= 0 || c.HTTP.MaxHeaderBytes <= 0 {
		return errors.New("HTTP size limits must be positive")
	}
	if c.Env != "production" {
		return nil
	}

	secureSSLModes := map[string]bool{"require": true, "verify-ca": true, "verify-full": true}
	if c.DB.URL == "" {
		if !secureSSLModes[strings.ToLower(c.DB.SSLMode)] {
			return errors.New("DB_SSLMODE must be require, verify-ca, or verify-full in production")
		}
	} else {
		databaseURL, err := url.Parse(c.DB.URL)
		if err != nil || (databaseURL.Scheme != "postgres" && databaseURL.Scheme != "postgresql") || databaseURL.Host == "" {
			return errors.New("DATABASE_URL must be a valid PostgreSQL URL")
		}
		if !secureSSLModes[strings.ToLower(databaseURL.Query().Get("sslmode"))] {
			return errors.New("DATABASE_URL must set sslmode=require, verify-ca, or verify-full in production")
		}
	}
	return nil
}

func validateRequired(values map[string]string) error {
	for name, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required in production", name)
		}
	}
	return nil
}

func loadDBConfig() DBConfig {
	return DBConfig{
		URL:      getEnv("DATABASE_URL", ""),
		Host:     getEnv("DB_HOST", ""),
		Port:     getEnv("DB_PORT", "5432"),
		User:     getEnv("DB_USER", "postgres"),
		Password: getEnv("DB_PASSWORD", ""),
		DBName:   getEnv("DB_NAME", "chakam"),
		SSLMode:  getEnv("DB_SSLMODE", "disable"),
	}
}



func loadWhatsAppConfig() WhatsAppConfig {
	return WhatsAppConfig{
		VerifyToken:   getEnv("WHATSAPP_VERIFY_TOKEN", ""),
		AppSecret:     getEnv("WHATSAPP_APP_SECRET", ""),
		AccessToken:   getEnv("WHATSAPP_ACCESS_TOKEN", ""),
		PhoneNumberID: getEnv("WHATSAPP_PHONE_NUMBER_ID", ""),
		BaseURL:       getEnv("WHATSAPP_BASE_URL", "https://graph.facebook.com/v25.0"),
	}
}

func loadRedisConfig() RedisConfig {
	return RedisConfig{
		Host:     getEnv("REDIS_HOST", "localhost"),
		Port:     getEnv("REDIS_PORT", "6379"),
		Username: getEnv("REDIS_USERNAME", ""),
		Password: getEnv("REDIS_PASSWORD", ""),
		DB:       getEnvInt("REDIS_DB", 0),
		TLS:      getEnvBool("REDIS_TLS", false),
	}
}

func loadHTTPConfig() HTTPConfig {
	addr := getEnv("SERVER_ADDR", "")
	if addr == "" {
		addr = ":" + getEnv("PORT", "8080")
	}
	return HTTPConfig{
		Addr:              addr,
		ReadTimeout:       getEnvDuration("HTTP_READ_TIMEOUT", 15*time.Second),
		ReadHeaderTimeout: getEnvDuration("HTTP_READ_HEADER_TIMEOUT", 5*time.Second),
		WriteTimeout:      getEnvDuration("HTTP_WRITE_TIMEOUT", 30*time.Second),
		IdleTimeout:       getEnvDuration("HTTP_IDLE_TIMEOUT", 60*time.Second),
		ShutdownTimeout:   getEnvDuration("HTTP_SHUTDOWN_TIMEOUT", 15*time.Second),
		MaxBodyBytes:      int64(getEnvInt("HTTP_MAX_BODY_BYTES", 1<<20)),
		MaxHeaderBytes:    getEnvInt("HTTP_MAX_HEADER_BYTES", 1<<20),
	}
}

func loadGeminiConfig() GeminiConfig {
	return GeminiConfig{
		Model:  getEnv("GEMINI_MODEL", "gemini-2.5-flash"),
		APIKey: getEnv("GEMINI_API_KEY", ""),
	}
}
