package mercure

import (
	"context"
	"log/slog"
)

type mercureHandler struct {
	innerHandler slog.Handler
}

func (m mercureHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return m.innerHandler.Enabled(ctx, level)
}

func (m mercureHandler) Handle(ctx context.Context, record slog.Record) error {
	var attrs []slog.Attr

	if u, ok := ctx.Value(UpdateContextKey).(*Update); ok {
		attrs = append(attrs, slog.Any("update", u))
	}

	if s, ok := ctx.Value(SubscriberContextKey).(*Subscriber); ok {
		attrs = append(attrs, slog.Any("subscriber", s))
	}

	record.AddAttrs(attrs...)

	return m.innerHandler.Handle(ctx, record)
}

func (m mercureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return m.innerHandler.WithAttrs(attrs)
}

func (m mercureHandler) WithGroup(name string) slog.Handler {
	return m.innerHandler.WithGroup(name)
}
