package logger

import (
	"context"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func Trace(ctx context.Context, op string, fields ...zap.Field) func(...zap.Field) {
	l := From(ctx)
	if !l.Core().Enabled(zapcore.DebugLevel) {
		return func(...zap.Field) {}
	}

	l.Debug(op+" start", fields...)
	start := time.Now()

	return func(extra ...zap.Field) {
		out := make([]zap.Field, 0, len(fields)+len(extra)+1)
		out = append(out, fields...)
		out = append(out, extra...)
		out = append(out, zap.Duration("took", time.Since(start)))
		l.Debug(op+" done", out...)
	}
}
