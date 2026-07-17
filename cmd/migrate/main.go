// Command migrate applies the SQL migrations in internal/db/migrations against
// the database from DATABASE_URL (or .env.local). It wraps golang-migrate as a
// library so the Makefile does not have to fetch the upstream CLI.
//
//	go run ./cmd/migrate up
//	go run ./cmd/migrate down [n|all]
//	go run ./cmd/migrate force <version>
//	go run ./cmd/migrate version
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"go.uber.org/zap"

	"github.com/klemanjar0/go-notifier-bot/internal/config"
	"github.com/klemanjar0/go-notifier-bot/internal/logger"
)

// migrateLogger adapts zap to golang-migrate's Logger interface.
type migrateLogger struct{ log *zap.Logger }

func (l migrateLogger) Printf(format string, v ...any) {
	l.log.Debug(strings.TrimRight(fmt.Sprintf(format, v...), "\n"))
}

// Verbose tells golang-migrate whether to bother producing the messages at all.
func (l migrateLogger) Verbose() bool { return l.log.Core().Enabled(zap.DebugLevel) }

func main() {
	log.SetFlags(0)

	path := flag.String("path", "internal/db/migrations", "directory holding the migration files")
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() == 0 {
		usage()
		log.Fatal("missing command")
	}

	if err := run(*path, flag.Arg(0), flag.Args()[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(path, command string, args []string) error {
	// Resolve the command before touching the database, so a typo fails
	// immediately instead of after a connection attempt.
	action, err := parse(command, args)
	if err != nil {
		return err
	}

	// Loads the dotenv files too, so LOG_LEVEL from .env.local reaches the
	// logger below.
	dbURL, err := config.LoadDatabaseURL()
	if err != nil {
		return err
	}
	if err := logger.Init(logger.Options{
		Level:  os.Getenv("LOG_LEVEL"),
		Format: os.Getenv("LOG_FORMAT"),
	}); err != nil {
		return err
	}
	defer logger.Sync()

	logger.Debug("migrating",
		zap.String("command", command),
		zap.String("path", path),
		zap.String("database_url", config.RedactURL(dbURL)),
	)

	m, err := migrate.New("file://"+path, pgxURL(dbURL))
	if err != nil {
		return fmt.Errorf("open migrator: %w", err)
	}
	// Routes golang-migrate's own chatter (which file it is applying, locking,
	// ...) through zap. Only visible at LOG_LEVEL=debug.
	m.Log = migrateLogger{logger.Named("migrate")}
	// Close reports both the source and the database error; neither is fatal to
	// a migration that already succeeded, so they are only worth logging.
	defer func() {
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			log.Printf("close migrator: source=%v db=%v", srcErr, dbErr)
		}
	}()

	return action(m)
}

// parse turns a command and its arguments into the migrator call they name,
// rejecting anything malformed up front.
func parse(command string, args []string) (func(*migrate.Migrate) error, error) {
	switch command {
	case "up":
		return func(m *migrate.Migrate) error { return report(m.Up()) }, nil

	case "down":
		if len(args) > 0 && args[0] == "all" {
			return func(m *migrate.Migrate) error { return report(m.Down()) }, nil
		}
		steps := 1
		if len(args) > 0 {
			n, err := strconv.Atoi(args[0])
			if err != nil {
				return nil, fmt.Errorf("down: %q is not a number of steps or \"all\"", args[0])
			}
			steps = n
		}
		if steps <= 0 {
			return nil, fmt.Errorf("down: steps must be positive, got %d", steps)
		}
		return func(m *migrate.Migrate) error { return report(m.Steps(-steps)) }, nil

	case "force":
		if len(args) == 0 {
			return nil, errors.New("force: missing version (make migrate-force VERSION=1)")
		}
		version, err := strconv.Atoi(args[0])
		if err != nil {
			return nil, fmt.Errorf("force: %q is not a version", args[0])
		}
		return func(m *migrate.Migrate) error { return m.Force(version) }, nil

	case "version":
		return printVersion, nil

	default:
		usage()
		return nil, fmt.Errorf("unknown command %q", command)
	}
}

func printVersion(m *migrate.Migrate) error {
	version, dirty, err := m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		fmt.Println("no migrations applied")
		return nil
	}
	if err != nil {
		return err
	}
	if dirty {
		fmt.Printf("%d (dirty)\n", version)
		return nil
	}
	fmt.Println(version)
	return nil
}

// report treats "nothing to do" as success, so `make migrate-up` on an
// up-to-date database is not an error.
func report(err error) error {
	if errors.Is(err, migrate.ErrNoChange) {
		fmt.Println("no change")
		return nil
	}
	if err != nil {
		return err
	}
	fmt.Println("ok")
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

func usage() {
	fmt.Print(`usage: go run ./cmd/migrate [-path dir] <command>

commands:
  up               apply all pending migrations
  down [n|all]     roll back the last n migrations (default 1)
  force <version>  set the schema version without running SQL
  version          print the current schema version
`)
}
