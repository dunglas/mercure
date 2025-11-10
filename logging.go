package mercure

import (
	"context"
	"fmt"
	"log/slog"
)

// INTERNAL: NewSlogHandler returns a log/slog.Handler that automatically appends "update" and "subscriber"
// context, if applicable.
//
// This function will be removed when https://github.com/caddyserver/caddy/pull/7346 will be available.
//
//nolint:godoclint
func NewSlogHandler(handler slog.Handler) slog.Handler {
	return &mercureHandler{handler}
}

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

	if err := m.innerHandler.Handle(ctx, record); err != nil {
		return fmt.Errorf("error while logging: %w", err)
	}

	return nil
}

func (m mercureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return m.innerHandler.WithAttrs(attrs)
}

func (m mercureHandler) WithGroup(name string) slog.Handler {
	return m.innerHandler.WithGroup(name)
}
