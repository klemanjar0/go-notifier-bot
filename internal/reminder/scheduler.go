package reminder

import (
	"context"
	"time"

	"github.com/go-telegram/bot"
	"github.com/klemanjar0/go-notifier-bot/internal/db"
	"go.uber.org/zap"
)

const pollInterval = 30 * time.Second
const maxStale = 6 * time.Hour

type Scheduler struct {
	q   *db.Queries
	bot *bot.Bot
	log *zap.Logger
}

func NewScheduler(q *db.Queries, b *bot.Bot, log *zap.Logger) *Scheduler {
	return &Scheduler{q: q, bot: b, log: log}
}

func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	s.log.Info("scheduler started", zap.Duration("interval", pollInterval))

	for {
		select {
		case <-ctx.Done():
			s.log.Info("scheduler stopped")
			return
		case <-ticker.C:
			s.processDue(ctx)
		}
	}
}

func (s *Scheduler) processDue(ctx context.Context) {
	due, err := s.q.GetDueReminders(ctx)
	if err != nil {
		s.log.Error("get due reminders", zap.Error(err))
		return
	}

	for _, r := range due {
		select {
		case <-ctx.Done():
			return
		default:
		}

		s.deliver(ctx, r)
	}
}

func (s *Scheduler) deliver(ctx context.Context, r db.Reminder) {
	if maxStale > 0 && time.Since(r.FireAt.Time) > maxStale {
		s.log.Warn("skipping stale reminder",
			zap.Int64("id", r.ID),
			zap.Time("fire_at", r.FireAt.Time),
		)
		if err := s.q.MarkSent(ctx, r.ID); err != nil {
			s.log.Error("mark stale sent", zap.Int64("id", r.ID), zap.Error(err))
		}
		return
	}

	_, err := s.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: r.ChatID,
		Text:   "🔔 " + r.Text,
	})
	if err != nil {
		s.log.Error("send reminder",
			zap.Int64("id", r.ID),
			zap.Int64("chat_id", r.ChatID),
			zap.Error(err),
		)
		return
	}

	if err := s.q.MarkSent(ctx, r.ID); err != nil {
		s.log.Error("mark sent", zap.Int64("id", r.ID), zap.Error(err))
	}
}
