package telegram

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"go.uber.org/zap"

	"github.com/klemanjar0/go-notifier-bot/internal/config"
	"github.com/klemanjar0/go-notifier-bot/internal/logger"
)

type TelegramBot struct {
	Bot *bot.Bot
	log *zap.Logger
}

func Init(cfg *config.Config) (*TelegramBot, error) {
	log := logger.Named("telegram")

	opts := []bot.Option{
		bot.WithDefaultHandler(handler),
		bot.WithMessageTextHandler("/start", bot.MatchTypeExact, StartHandler),
		bot.WithMessageTextHandler("/clear_all", bot.MatchTypeExact, AnythingHandler),
		bot.WithMessageTextHandler("/list", bot.MatchTypeExact, AnythingHandler),
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

func handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log := logger.Named("telegram")
	if update.Message == nil {
		log.Debug("update without message, ignoring", zap.Int64("update_id", update.ID))
		return
	}

	ctx = logger.Into(ctx, log.With(
		zap.Int64("update_id", update.ID),
		zap.Int64("chat_id", update.Message.Chat.ID),
	))
	log = logger.From(ctx)

	// Message text can be personal, so log its size rather than its content.
	done := logger.Trace(ctx, "telegram.handle_update", zap.Int("text_len", len(update.Message.Text)))
	defer done()

	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   update.Message.Text,
	}); err != nil {
		log.Error("send message failed", zap.Error(err))
		return
	}
	log.Debug("message sent")
}

func (bot *TelegramBot) Start(ctx context.Context) {
	bot.Bot.Start(ctx)
}

func (bot *TelegramBot) Shutdown(ctx context.Context) {
	bot.Bot.Close(ctx)
}
