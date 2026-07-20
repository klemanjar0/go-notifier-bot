package telegram

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	"go.uber.org/zap"

	"github.com/klemanjar0/go-notifier-bot/internal/config"
	"github.com/klemanjar0/go-notifier-bot/internal/logger"
)

type TelegramBot struct {
	Bot *bot.Bot
	log *zap.Logger
}

func Init(cfg *config.Config, parser Parser, reminders Reminders) (*TelegramBot, error) {
	log := logger.Named("telegram")
	handlers := NewHandlers(parser, reminders)

	opts := []bot.Option{
		bot.WithDefaultHandler(handlers.OnMessage),
		bot.WithMessageTextHandler("/start", bot.MatchTypeExact, handlers.StartHandler),
		bot.WithMessageTextHandler("/clear_all", bot.MatchTypeExact, handlers.ClearAllHandler),
		bot.WithMessageTextHandler("/list", bot.MatchTypeExact, handlers.ListHandler),
		bot.WithCallbackQueryDataHandler("cancel:", bot.MatchTypePrefix, handlers.CancelCallbackHandler),
		bot.WithDebugHandler(func(format string, args ...any) {
			log.Debug(fmt.Sprintf(format, args...))
		}),
		bot.WithErrorsHandler(func(err error) {
			log.Error("bot error", zap.Error(err))
		}),
	}

	if log.Core().Enabled(zap.DebugLevel) {
		opts = append(opts, bot.WithDebug())
	}

	b, err := bot.New(cfg.TelegramKey, opts...)
	if err != nil {
		return nil, fmt.Errorf("telegram: new bot: %w", err)
	}

	log.Info("bot initialised")
	return &TelegramBot{Bot: b, log: log}, nil
}

func (bot *TelegramBot) Start(ctx context.Context) {
	bot.Bot.Start(ctx)
}

func (bot *TelegramBot) Shutdown(ctx context.Context) {
	bot.Bot.Close(ctx)
}
