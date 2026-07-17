package reminder

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/klemanjar0/go-notifier-bot/internal/db"
)

// Store is the subset of the sqlc-generated *db.Queries that the service needs.
// Declaring it as an interface keeps the service testable without a real
// database and documents exactly which queries it relies on.
type Store interface {
	CreateReminder(ctx context.Context, arg db.CreateReminderParams) (db.Reminder, error)
	ListPending(ctx context.Context, chatID int64) ([]db.Reminder, error)
}

// Service creates and reads reminders. It owns the persistence concern so the
// telegram handlers stay free of database details.
type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

// Create stores a reminder for chatID that should fire at fireAt and returns
// its database id.
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

// ListPending returns the not-yet-sent reminders for a chat, soonest first.
func (s *Service) ListPending(ctx context.Context, chatID int64) ([]db.Reminder, error) {
	items, err := s.store.ListPending(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("reminder: list pending: %w", err)
	}
	return items, nil
}
