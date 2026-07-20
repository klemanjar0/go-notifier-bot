package db

import (
	"embed"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// migrationsFS carries the SQL migration files into the binary so the bot can
// migrate itself on startup, without shipping the .sql files or a shell to the
// runtime image. The standalone cmd/migrate tool still reads them from disk for
// local development (down/force/version), which the embedded path does not need.
//
//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate applies all pending up migrations against dbURL. It is safe to call
// on every startup: an already-migrated database reports no change and returns
// nil.
func Migrate(dbURL string) error {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("migrate: open embedded source: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, pgxURL(dbURL))
	if err != nil {
		return fmt.Errorf("migrate: init: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate: up: %w", err)
	}
	return nil
}

// pgxURL rewrites the postgres:// URL used everywhere else in the project to the
// pgx5:// scheme that golang-migrate's pgx/v5 driver registers itself under.
func pgxURL(dbURL string) string {
	for _, scheme := range []string{"postgres://", "postgresql://"} {
		if rest, ok := strings.CutPrefix(dbURL, scheme); ok {
			return "pgx5://" + rest
		}
	}
	return dbURL
}
