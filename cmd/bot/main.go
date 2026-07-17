package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/klemanjar0/go-notifier-bot/internal/config"
	"github.com/klemanjar0/go-notifier-bot/internal/logger"
	"github.com/klemanjar0/go-notifier-bot/internal/telegram"
)

func main() {
	ctx := context.Background()

	var appBot *telegram.TelegramBot
	cfg, err := config.Load()

	if err != nil {
		log.Fatal(err)
	}
	if err := logger.Init(logger.Options{Level: cfg.LogLevel, Format: cfg.LogFormat}); err != nil {
		log.Fatal(err)
	}
	defer logger.Sync()

	logger.Info("starting notifier bot", zap.String("log_level", cfg.LogLevel))
	logger.Debug("config loaded", zap.String("database_url", cfg.SafeDatabaseURL()))

	// remove for prod
	logger.Debug("telegram api key", zap.String("api_key", cfg.TelegramKey))

	if appBot, err = telegram.Init(cfg); err != nil {
		logger.Fatal("telegram init failed", zap.Error(err))
	}

	appBot.Start(ctx)

	logger.Info("started")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		appBot.Start(ctx)
	}()

	<-quit
	logger.Info("shutting down user service")
	appBot.Shutdown(ctx)
}
