package logger

import (
	"context"
	"log/slog"
	"os"
)

type ctxKey struct{}

var DefaultLogger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
	Level: slog.LevelInfo,
}))

func WithLogger(ctx context.Context, l *slog.Logger) context.Context {
	if l == nil {
		return ctx
	}

	return context.WithValue(ctx, ctxKey{}, l)
}

func FromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return DefaultLogger
	}

	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok && l != nil {
		return l
	}

	DefaultLogger.WarnContext(ctx, "No logger found in context, using default logger")
	return DefaultLogger
}

func Info(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).InfoContext(ctx, msg, args...)
}

func Warn(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).WarnContext(ctx, msg, args...)
}

func Error(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).ErrorContext(ctx, msg, args...)
}

func Debug(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).DebugContext(ctx, msg, args...)
}
