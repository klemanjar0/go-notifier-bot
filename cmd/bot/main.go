package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"google.golang.org/genai"

	"github.com/klemanjar0/go-notifier-bot/internal/config"
	"github.com/klemanjar0/go-notifier-bot/internal/db"
	"github.com/klemanjar0/go-notifier-bot/internal/health"
	"github.com/klemanjar0/go-notifier-bot/internal/logger"
	"github.com/klemanjar0/go-notifier-bot/internal/parser"
	"github.com/klemanjar0/go-notifier-bot/internal/reminder"
	"github.com/klemanjar0/go-notifier-bot/internal/telegram"
)

func main() {
	ctx := context.Background()

	var appBot *telegram.TelegramBot
	cfg, err := config.Load()

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  cfg.GeminiKey,
		Backend: genai.BackendGeminiAPI,
	})

	if err != nil {
		log.Fatal(err)
	}

	p, err := parser.NewGeminiParser(client)

	if err != nil {
		log.Fatal(err)
	}

	if err := logger.Init(logger.Options{Level: cfg.LogLevel, Format: cfg.LogFormat}); err != nil {
		log.Fatal(err)
	}
	defer logger.Sync()

	logger.Info("starting notifier bot", zap.String("log_level", cfg.LogLevel))
	logger.Debug("config loaded", zap.String("database_url", cfg.SafeDatabaseURL()))

	// Apply pending migrations before opening the app pool, so the schema the
	// queries expect is guaranteed to exist. Embedded, so the runtime image
	// needs no .sql files and no shell.
	if err := db.Migrate(cfg.DatabaseURL); err != nil {
		logger.Fatal("migrations failed", zap.Error(err))
	}
	logger.Info("migrations applied")

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("database connect failed", zap.Error(err))
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		logger.Fatal("database ping failed", zap.Error(err))
	}

	queries := db.New(pool)
	reminders := reminder.NewService(queries)

	// remove for prod
	logger.Debug("telegram api key", zap.String("api_key", cfg.TelegramKey))

	if appBot, err = telegram.Init(cfg, p, reminders); err != nil {
		logger.Fatal("telegram init failed", zap.Error(err))
	}

	sched := reminder.NewScheduler(queries, appBot.Bot, logger.Named("scheduler"))

	healthSrv := health.NewServer(cfg.HealthAddr)
	go healthSrv.Start()

	go sched.Run(ctx)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		appBot.Start(ctx)
	}()

	logger.Info("started")

	<-quit
	logger.Info("shutting down user service")
	appBot.Shutdown(ctx)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	healthSrv.Shutdown(shutdownCtx)
}
