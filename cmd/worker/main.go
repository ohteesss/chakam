package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"chakam/internal/application/agreement"
	"chakam/internal/application/conversation"
	"chakam/internal/application/session"
	"chakam/internal/application/user"
	"chakam/internal/config"
	"chakam/internal/infrastructure/database"
	"chakam/internal/infrastructure/gemini"
	"chakam/internal/infrastructure/queue"
	"chakam/internal/infrastructure/repository"
	"chakam/internal/infrastructure/whatsapp"
	"chakam/internal/interfaces/jobs"
	"chakam/internal/pkg/logger"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	log := logger.New()
	if err := run(log); err != nil {
		log.Error("worker stopped with an error", zap.Error(err))
		_ = log.Sync()
		os.Exit(1)
	}
	_ = log.Sync()
}

func run(log *zap.Logger) error {
	cfg := config.Load()
	if err := cfg.ValidateWorker(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	log.Info("starting Chakam worker",
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

	userRepo := repository.NewGormUserRepository(db)
	userService := user.NewService(userRepo)
	sessionRepo := repository.NewGormSessionRepository(db)
	sessionService := session.NewService(sessionRepo)
	agreementService := agreement.NewService(repository.NewGormAgreementRepository(db))
	whatsAppClient := whatsapp.NewClient(
		cfg.WhatsApp.AccessToken,
		cfg.WhatsApp.PhoneNumberID,
		cfg.WhatsApp.BaseURL,
	)

	signalCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	geminiClient, err := gemini.NewClient(signalCtx, cfg.Gemini.APIKey, cfg.Gemini.Model)
	if err != nil {
		return fmt.Errorf("create Gemini client: %w", err)
	}
	functionRegistry := conversation.NewFunctionRegistry(agreementService, userService, whatsAppClient)
	orchestrator := conversation.NewLLMOrchestrator(geminiClient, sessionService, userService, functionRegistry)
	messageHandler := queue.NewMessageHandler(log, userService, agreementService, whatsAppClient, orchestrator)

	server := asynq.NewServer(queue.RedisClientOpt(cfg.Redis), asynq.Config{
		Concurrency: 10,
		Queues: map[string]int{
			"critical": 6,
			"default":  3,
			"low":      1,
		},
	})
	mux := asynq.NewServeMux()
	mux.HandleFunc(queue.TypeProcessMessage, messageHandler.HandleProcessMessage)
	mux.HandleFunc(queue.TypeSendMessage, messageHandler.HandleSendMessage)
	if err := server.Start(mux); err != nil {
		return fmt.Errorf("start queue worker: %w", err)
	}

	scheduler := jobs.NewScheduler(log)
	scheduler.Register(jobs.NewExpireSessions(log, sessionRepo, 5*time.Minute))
	schedulerDone := make(chan struct{})
	go func() {
		defer close(schedulerDone)
		scheduler.Run(signalCtx)
	}()

	log.Info("worker ready")
	<-signalCtx.Done()
	stop()
	log.Info("worker shutdown signal received")
	server.Shutdown()
	<-schedulerDone
	log.Info("worker stopped cleanly")
	return nil
}
