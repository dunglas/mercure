package mercure

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel/trace"
)

type updateContextKeyType struct{}

var UpdateContextKey updateContextKeyType //nolint:gochecknoglobals

// reservedTopicSubstring is the substring whose presence in any topic
// (canonical or alternate) identifies the hub's own resources, such as
// subscription events. Publishers must not be able to forge those.
const reservedTopicSubstring = "/.well-known/mercure"

// Sentinel errors returned by Publish for invalid Update payloads.
// PublishHandler maps them to HTTP status codes; library callers can
// inspect them with errors.Is.
var (
	// ErrReservedTopic is returned when an Update contains a topic
	// whose value references the reserved "/.well-known/mercure"
	// namespace, which only the hub itself is allowed to publish to.
	ErrReservedTopic = errors.New(`topic value references the reserved "/.well-known/mercure" namespace`)

	// ErrInvalidEventID is returned when an Update's event id contains
	// a character that would let the publisher inject arbitrary SSE
	// fields into the subscriber's stream.
	ErrInvalidEventID = errors.New(`"id" field contains a forbidden control character`)

	// ErrInvalidEventType is returned when an Update's event type
	// contains a character that would let the publisher inject
	// arbitrary SSE fields into the subscriber's stream.
	ErrInvalidEventType = errors.New(`"type" field contains a forbidden control character`)
)

// sseFieldForbiddenChars are characters that, if copied into an SSE
// header field such as id: or event:, would let a publisher inject
// arbitrary SSE fields into subscribers' streams.
const sseFieldForbiddenChars = "\x00\r\n"

// validate enforces the publish-side input rules that protect
// subscribers from update forgery and SSE field injection. It is the
// single source of truth for these checks; PublishHandler does not
// re-validate. The sentinel errors returned (ErrReservedTopic,
// ErrInvalidEventID, ErrInvalidEventType) are exported so callers of
// Hub.Publish can branch on them via errors.Is.
func (u *Update) validate() error {
	for _, t := range u.Topics {
		if strings.Contains(t, reservedTopicSubstring) {
			return fmt.Errorf("%q: %w", t, ErrReservedTopic)
		}
	}

	if strings.ContainsAny(u.ID, sseFieldForbiddenChars) {
		return ErrInvalidEventID
	}

	if strings.ContainsAny(u.Type, sseFieldForbiddenChars) {
		return ErrInvalidEventType
	}

	return nil
}

// Publish broadcasts the given update to all subscribers.
// The id field of the Update instance can be updated by the underlying Transport.
func (h *Hub) Publish(ctx context.Context, update *Update) error {
	ctx, span := startSpan(ctx, "mercure.publish", trace.WithSpanKind(trace.SpanKindProducer))
	// Deferred so the ID assigned by the transport via AssignUUID lands on the span.
	defer func() {
		if span.IsRecording() {
			span.SetAttributes(update.SpanAttributes()...)
		}

		span.End()
	}()

	if err := update.validate(); err != nil {
		if h.logger.Enabled(ctx, slog.LevelInfo) {
			h.logger.LogAttrs(ctx, slog.LevelInfo, "Rejected invalid update", slog.Any("error", err))
		}

		recordSpanError(span, err)

		return err
	}

	ctx = context.WithValue(ctx, UpdateContextKey, update)

	if err := h.transport.Dispatch(ctx, update); err != nil {
		if h.logger.Enabled(ctx, slog.LevelError) {
			h.logger.LogAttrs(ctx, slog.LevelError, "Failed to dispatch update", slog.Any("error", err))
		}

		recordSpanError(span, err)

		return err //nolint:wrapcheck
	}

	h.metrics.UpdatePublished(update)

	if h.logger.Enabled(ctx, slog.LevelDebug) {
		h.logger.LogAttrs(ctx, slog.LevelDebug, "Update published")
	}

	return nil
}

// PublishHandler allows publisher to broadcast updates to all subscribers.
//
//nolint:funlen,gocognit
func (h *Hub) PublishHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := startSpan(r.Context(), "mercure.publish", trace.WithSpanKind(trace.SpanKindProducer))

	var u *Update
	// Deferred so the ID assigned by the transport via AssignUUID lands on the span.
	defer func() {
		if u != nil && span.IsRecording() {
			span.SetAttributes(u.SpanAttributes()...)
		}

		span.End()
	}()

	r = r.WithContext(ctx)

	var claims *claims

	if h.publisherJWTKeyFunc != nil {
		var err error

		claims, err = h.authorize(r, true)
		if err != nil || claims == nil || claims.Mercure.Publish == nil {
			h.httpAuthorizationError(w, r, err)

			if err != nil {
				recordSpanError(span, err)
			}

			return
		}
	}

	if r.ParseForm() != nil { //nolint:gosec // body size can be limited using Caddy's request_body directive
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)

		return
	}

	topics := r.PostForm["topic"]
	if len(topics) == 0 {
		http.Error(w, `Missing "topic" parameter`, http.StatusBadRequest)

		return
	}

	var retry uint64

	if retryString := r.PostForm.Get("retry"); retryString != "" {
		var err error
		if retry, err = strconv.ParseUint(retryString, 10, 64); err != nil {
			http.Error(w, `Invalid "retry" parameter`, http.StatusBadRequest)

			return
		}
	}

	private := len(r.PostForm["private"]) != 0
	if claims != nil && !canDispatch(h.topicSelectorStore, topics, claims.Mercure.Publish) { //nolint:nestif
		if private {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)

			return
		}

		infoEnabled := h.logger.Enabled(ctx, slog.LevelInfo)
		if h.isBackwardCompatiblyEnabledWith(7) {
			if infoEnabled {
				h.logger.LogAttrs(ctx, slog.LevelInfo, `Deprecated: posting public updates to topics not listed in the "mercure.publish" JWT claim is deprecated since the version 7 of the protocol, use '["*"]' as value to allow publishing on all topics.`)
			}
		} else {
			if infoEnabled {
				h.logger.LogAttrs(ctx, slog.LevelInfo, `Unsupported: posting public updates to topics not listed in the "mercure.publish" JWT claim is not supported anymore, use '["*"]' as value to allow publishing on all topics or enable backward compatibility with the version 7 of the protocol.`)
			}

			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)

			return
		}
	}

	u = &Update{
		Topics:  topics,
		Private: private,
		Debug:   h.debug,
		Event:   Event{r.PostForm.Get("data"), r.PostForm.Get("id"), r.PostForm.Get("type"), retry},
	}

	dispatchCtx := context.WithoutCancel(ctx)

	// Validation, dispatch, logging and metrics live in Hub.Publish.
	if err := h.Publish(dispatchCtx, u); err != nil {
		switch {
		case errors.Is(err, ErrReservedTopic):
			http.Error(w, err.Error(), http.StatusForbidden)
		case errors.Is(err, ErrInvalidEventID), errors.Is(err, ErrInvalidEventType):
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}

		// Mirror the error onto the handler span too; Hub.Publish's child
		// span already records it, but leaving the parent span as success
		// is misleading.
		recordSpanError(span, err)

		return
	}

	if _, err := io.WriteString(w, u.ID); err != nil {
		if h.logger.Enabled(ctx, slog.LevelInfo) {
			h.logger.LogAttrs(ctx, slog.LevelInfo, "Failed to write publish response", slog.Any("error", err))
		}

		return
	}
}
