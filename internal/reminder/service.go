package reminder

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/klemanjar0/go-notifier-bot/internal/db"
)

type Store interface {
	CreateReminder(ctx context.Context, arg db.CreateReminderParams) (db.Reminder, error)
	ListPending(ctx context.Context, chatID int64) ([]db.Reminder, error)
	MarkAllSentForChat(ctx context.Context, chatID int64) error
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) Create(ctx context.Context, chatID int64, text string, fireAt time.Time) (int64, error) {
	r, err := s.store.CreateReminder(ctx, db.CreateReminderParams{
		ChatID: chatID,
		Text:   text,
		FireAt: pgtype.Timestamptz{Time: fireAt, Valid: true},
	})
	if err != nil {
		return 0, fmt.Errorf("reminder: create: %w", err)
	}
	return r.ID, nil
}

func (s *Service) ListPending(ctx context.Context, chatID int64) ([]db.Reminder, error) {
	items, err := s.store.ListPending(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("reminder: list pending: %w", err)
	}
	return items, nil
}

func (s *Service) MarkAllSentForChat(ctx context.Context, chatID int64) error {
	err := s.store.MarkAllSentForChat(ctx, chatID)
	if err != nil {
		return fmt.Errorf("reminder: clear all: %w", err)
	}
	return nil
}
