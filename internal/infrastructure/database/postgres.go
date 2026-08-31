// Package database opens and configures the Postgres connection. It's the only
// place that knows how to turn config settings into a live *gorm.DB.
package database

import (
	"context"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"chakam/internal/config"
)

// Connect opens a GORM connection to Postgres and verifies it actually works.
//
// It takes a config.DBConfig (plain settings) and returns a live *gorm.DB. The
// caller (main.go) owns the result and injects it into repositories — config
// stays a pure value type, the connection lives here.
//
// logSQL toggles GORM's query logging: in dev we want to SEE every SQL
// statement; in prod that can leak message content or phone numbers, so SQL
// logging is disabled and errors are returned to the application logger.
func Connect(cfg config.DBConfig, logSQL bool) (*gorm.DB, error) {
	// The DSN (Data Source Name) is the connection string. We build it from
	// config rather than hardcoding, and use key=value form because it reads
	// clearly and handles empty fields gracefully.
	dsn := cfg.URL
	if dsn == "" {
		dsn = fmt.Sprintf(
			"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s connect_timeout=5",
			cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
		)
	}

	// Choose the log level. Info logs ALL SQL (with timing + rows affected).
	// GORM's logger writes its own
	// human-readable, colorized lines to stdout — separate from our zap logs,
	// which is fine (and more readable) for dev.
	logLevel := gormlogger.Silent
	if logSQL {
		logLevel = gormlogger.Info
	}

	// gorm.Open only PARSES the DSN and prepares a connection pool — it does NOT
	// guarantee the database is reachable yet. So a nil error here is not proof
	// the DB is up; that's what the Ping below is for.
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("database: open: %w", err)
	}

	// GORM wraps the standard library's *sql.DB. We reach through to it to (a)
	// tune the connection pool and (b) actually Ping the server.
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("database: get sql.DB: %w", err)
	}

	// Connection-pool settings. Without these you get database/sql's defaults,
	// which allow UNLIMITED open connections — under load that can exhaust
	// Postgres's max_connections. Bounding them is standard production hygiene.
	sqlDB.SetMaxOpenConns(25)                 // hard cap on concurrent connections
	sqlDB.SetMaxIdleConns(25)                 // keep this many warm to avoid reconnect churn
	sqlDB.SetConnMaxLifetime(5 * time.Minute) // recycle connections so none lives forever

	// Ping forces a real round-trip — this is where a wrong password / down DB
	// actually surfaces. Failing fast at startup beats failing on the first
	// request.
	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(pingCtx); err != nil {
		return nil, fmt.Errorf("database: ping: %w", err)
	}

	return db, nil
}
