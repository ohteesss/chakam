package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"chakam/internal/application/agreement"
	"chakam/internal/application/user"
	"chakam/internal/config"
	"chakam/internal/infrastructure/database"
	"chakam/internal/infrastructure/queue"
	"chakam/internal/infrastructure/repository"
	"chakam/internal/interfaces/http/handler"
	"chakam/internal/interfaces/http/router"
	"chakam/internal/pkg/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	log := logger.New()
	if err := run(log); err != nil {
		log.Error("API server stopped with an error", zap.Error(err))
		_ = log.Sync()
		os.Exit(1)
	}
	_ = log.Sync()
}

func run(log *zap.Logger) error {
	cfg := config.Load()
	if err := cfg.ValidateAPI(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	log.Info("starting Chakam API server",
		zap.String("environment", cfg.Env),
		zap.String("version", version),
		zap.String("commit", commit),
	)

	db, err := database.Connect(cfg.DB, cfg.IsDev())
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get database pool: %w", err)
	}
	defer sqlDB.Close()

	queueClient := queue.NewClient(cfg.Redis)
	defer queueClient.Close()

	agreementService := agreement.NewService(repository.NewGormAgreementRepository(db))
	userService := user.NewService(repository.NewGormUserRepository(db))
	webhookHandler := handler.NewWebhookHandler(
		log,
		cfg.WhatsApp.VerifyToken,
		cfg.WhatsApp.AppSecret,
		queueClient,
		agreementService,
		userService,
	)
	apiHandler := handler.NewApiHandler(agreementService, userService)
	healthHandler := handler.NewHealthHandler(sqlDB, queueClient)

	if !cfg.IsDev() {
		gin.SetMode(gin.ReleaseMode)
	}
	engine := router.SetupRouter(log, cfg.HTTP.MaxBodyBytes)
	router.SetupRoutes(engine, webhookHandler, apiHandler, healthHandler, cfg.Security.InternalAPIKey)

	server := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           engine,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
		MaxHeaderBytes:    cfg.HTTP.MaxHeaderBytes,
	}
	serverErrors := make(chan error, 1)
	go func() {
		log.Info("API server listening", zap.String("addr", server.Addr))
		serverErrors <- server.ListenAndServe()
	}()

	signalCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	case <-signalCtx.Done():
	}

	stop()
	log.Info("shutdown signal received")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		_ = server.Close()
		return fmt.Errorf("graceful HTTP shutdown: %w", err)
	}
	log.Info("API server stopped cleanly")
	return nil
}
