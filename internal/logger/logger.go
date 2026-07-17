// Package logger wraps zap so every package in the project can log without
// threading a logger through constructors:
//
//	logger.Debug("reminder scheduled", zap.Int64("chat_id", id))
//
// Init installs the real logger at startup; until then every call goes to a
// no-op logger, so tests and library code never panic on an uninitialised
// global. Components that log a lot should take a Named logger, and request-
// scoped code should carry one in the context (see Into and From).
package logger

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Options controls how the global logger renders. Both fields are usually
// filled from config, which reads LOG_LEVEL and LOG_FORMAT.
type Options struct {
	// Level is debug, info, warn, or error.
	Level string
	// Format is console (human-readable, colourised) or json (for shipping).
	Format string
}

var (
	mu sync.RWMutex
	// base is what L() and Named() hand out; its caller info points at the code
	// that calls them.
	base = zap.NewNop()
	// wrapped is the same logger with one extra frame skipped, so the caller
	// reported by the package-level Debug/Info/... helpers is the code that
	// called *them*, not this file.
	wrapped = zap.NewNop()
)

// Init builds the global logger from opts. Call it once, early in main, and
// pair it with `defer logger.Sync()`.
func Init(opts Options) error {
	level, err := parseLevel(opts.Level)
	if err != nil {
		return err
	}
	encoder, err := newEncoder(opts.Format)
	if err != nil {
		return err
	}

	core := zapcore.NewCore(encoder, zapcore.Lock(os.Stderr), level)
	l := zap.New(core,
		zap.AddCaller(),
		// Errors are where a stack trace earns its noise; below that it just
		// buries the message.
		zap.AddStacktrace(zapcore.ErrorLevel),
	)

	mu.Lock()
	defer mu.Unlock()
	base = l
	wrapped = l.WithOptions(zap.AddCallerSkip(1))
	return nil
}

// L returns the global logger.
func L() *zap.Logger {
	mu.RLock()
	defer mu.RUnlock()
	return base
}

// Named returns the global logger tagged with a component name, e.g.
// logger.Named("telegram"). The name shows up in every line it writes, which
// makes debug output greppable per component.
func Named(name string) *zap.Logger {
	return L().Named(name)
}

// Sync flushes buffered entries. Safe to defer in main; the sync error on a
// terminal stderr is a known no-op quirk, so callers usually ignore it.
func Sync() error {
	return L().Sync()
}

type ctxKey struct{}

// Into returns a context carrying l, so downstream code can log with the same
// fields (chat id, update id, ...) without passing them along by hand.
func Into(ctx context.Context, l *zap.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// With is Into plus fields: the common case of pinning identifiers to
// everything logged under this context.
func With(ctx context.Context, fields ...zap.Field) context.Context {
	return Into(ctx, From(ctx).With(fields...))
}

// From returns the logger stored in ctx, or the global one if there is none.
// It never returns nil.
func From(ctx context.Context) *zap.Logger {
	if ctx != nil {
		if l, ok := ctx.Value(ctxKey{}).(*zap.Logger); ok && l != nil {
			return l
		}
	}
	return L()
}

// Package-level shorthands for the global logger.

func Debug(msg string, fields ...zap.Field) { logw().Debug(msg, fields...) }
func Info(msg string, fields ...zap.Field)  { logw().Info(msg, fields...) }
func Warn(msg string, fields ...zap.Field)  { logw().Warn(msg, fields...) }
func Error(msg string, fields ...zap.Field) { logw().Error(msg, fields...) }

// Fatal logs at error level and exits the process with status 1.
func Fatal(msg string, fields ...zap.Field) { logw().Fatal(msg, fields...) }

func logw() *zap.Logger {
	mu.RLock()
	defer mu.RUnlock()
	return wrapped
}

func parseLevel(level string) (zapcore.Level, error) {
	if strings.TrimSpace(level) == "" {
		return zapcore.InfoLevel, nil
	}
	parsed, err := zapcore.ParseLevel(strings.ToLower(strings.TrimSpace(level)))
	if err != nil {
		return 0, fmt.Errorf("logger: bad level %q (want debug, info, warn, or error)", level)
	}
	return parsed, nil
}

func newEncoder(format string) (zapcore.Encoder, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "console":
		cfg := zap.NewDevelopmentEncoderConfig()
		cfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		cfg.EncodeTime = zapcore.TimeEncoderOfLayout("15:04:05.000")
		return zapcore.NewConsoleEncoder(cfg), nil
	case "json":
		cfg := zap.NewProductionEncoderConfig()
		cfg.EncodeTime = zapcore.ISO8601TimeEncoder
		return zapcore.NewJSONEncoder(cfg), nil
	default:
		return nil, fmt.Errorf("logger: bad format %q (want console or json)", format)
	}
}
