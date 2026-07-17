package logger

import (
	"context"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// observe swaps the globals for a recording logger at the given level and
// restores them afterwards.
func observe(t *testing.T, level zapcore.Level) *observer.ObservedLogs {
	t.Helper()

	core, logs := observer.New(level)
	l := zap.New(core)

	mu.Lock()
	prevBase, prevWrapped := base, wrapped
	base, wrapped = l, l
	mu.Unlock()

	t.Cleanup(func() {
		mu.Lock()
		base, wrapped = prevBase, prevWrapped
		mu.Unlock()
	})
	return logs
}

func TestInitRejectsBadOptions(t *testing.T) {
	if err := Init(Options{Level: "chatty"}); err == nil {
		t.Error("bad level accepted")
	}
	if err := Init(Options{Level: "debug", Format: "yaml"}); err == nil {
		t.Error("bad format accepted")
	}
	// Empty means "use the defaults", not an error.
	if err := Init(Options{}); err != nil {
		t.Errorf("empty options rejected: %v", err)
	}
}

func TestFromReturnsGlobalWithoutContextLogger(t *testing.T) {
	if From(context.Background()) != L() {
		t.Error("From should fall back to the global logger")
	}
	// A nil context must not panic either; the linters rightly object to a
	// literal nil here, so go through a variable.
	var nilCtx context.Context
	if From(nilCtx) == nil {
		t.Error("From(nil) should return a usable logger")
	}
}

func TestWithAccumulatesFields(t *testing.T) {
	logs := observe(t, zapcore.DebugLevel)

	ctx := With(context.Background(), zap.Int64("chat_id", 7))
	ctx = With(ctx, zap.Int64("update_id", 9))
	From(ctx).Info("hello")

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields["chat_id"] != int64(7) || fields["update_id"] != int64(9) {
		t.Errorf("fields not carried through context: %v", fields)
	}
}

func TestTraceLogsStartAndDoneWithDuration(t *testing.T) {
	logs := observe(t, zapcore.DebugLevel)

	done := Trace(context.Background(), "op", zap.String("kind", "test"))
	done(zap.Bool("ok", true))

	entries := logs.All()
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want start and done", len(entries))
	}
	if entries[0].Message != "op start" || entries[1].Message != "op done" {
		t.Errorf("unexpected messages: %q, %q", entries[0].Message, entries[1].Message)
	}

	fields := entries[1].ContextMap()
	if _, ok := fields["took"]; !ok {
		t.Error("done entry missing took")
	}
	// The done entry carries both the fields from Trace and the ones passed on exit.
	if fields["kind"] != "test" || fields["ok"] != true {
		t.Errorf("done entry lost fields: %v", fields)
	}
}

func TestTraceIsNoopBelowDebug(t *testing.T) {
	logs := observe(t, zapcore.InfoLevel)

	done := Trace(context.Background(), "op")
	done()

	if n := logs.Len(); n != 0 {
		t.Errorf("Trace logged %d entries at info level, want 0", n)
	}
}
