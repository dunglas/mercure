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
	"unicode/utf8"

	"go.opentelemetry.io/otel/trace"
)

type updateContextKeyType struct{}

var UpdateContextKey updateContextKeyType //nolint:gochecknoglobals

// The reserved-namespace test itself lives in reservedtopic.go.

// Element-count caps prevent DoS amplification when callers fit many
// topics/matchers inside a request whose byte size is within transport
// limits (Caddy request_body, Go MaxHeaderBytes).
const (
	maxClaimMatchers = 1000 // mercure.subscribe / mercure.publish array
	maxPublishTopics = 1000 // "topic" form fields on publish
	// Subscribe-side matcher count is capped by maxMatcherCount
	// (subscribematchers.go).
)

// Sentinel errors returned by Publish. Callers can branch on them via
// errors.Is.
var (
	ErrReservedTopic     = errors.New(`topic value resolves into the reserved "/.well-known/mercure" namespace`)
	ErrReservedWildcard  = errors.New(`topic value "*" is reserved for the wildcard matcher and cannot be published`)
	ErrInvalidEventID    = errors.New(`"id" field contains a forbidden control character or invalid UTF-8, starts with "#", or is the reserved value "earliest"`)
	ErrInvalidEventType  = errors.New(`"type" field contains a forbidden control character or invalid UTF-8`)
	ErrReservedEventType = errors.New(`"type" field uses the reserved value "mercure"`)
	ErrInvalidTopic      = errors.New("topic contains a forbidden control character or invalid UTF-8")
	ErrTooManyTopics     = errors.New("too many topics in update")
	ErrInvalidData       = errors.New(`"data" field is not valid UTF-8`)
)

// Validate enforces the publish-side input rules that protect subscribers
// from update forgery and SSE field injection. Hub.Publish calls it, so the
// bundled hub and PublishHandler are already covered.
//
// A caller that builds an Update from untrusted input (e.g. a publisher
// request) and dispatches it through a Transport directly, bypassing
// Hub.Publish, MUST call Validate first and reject the update on error.
// Skipping it lets a CR, LF, or NUL in ID or Type inject arbitrary SSE
// fields into subscribers' streams (CWE-93). Validate also rejects the
// reserved "/.well-known/mercure" topic namespace, so it is meant for
// publisher input, not hub-internal updates such as subscription events.
func (u *Update) Validate() error {
	topics := u.topics()
	if len(topics) > maxPublishTopics {
		return ErrTooManyTopics
	}

	for _, t := range topics {
		// Control characters are forbidden by the protocol; a NUL would also
		// collide with the match cache's topic-list separator.
		if !validProtocolString(t) {
			return fmt.Errorf("%q: %w", t, ErrInvalidTopic)
		}

		if addressesReservedNamespace(t) {
			return fmt.Errorf("%q: %w", t, ErrReservedTopic)
		}

		// "*" is the reserved wildcard matcher pattern, so a topic literally
		// equal to "*" is not addressable by an Exact subscription; reject it
		// at publication rather than dispatch an unreachable update.
		if t == "*" {
			return fmt.Errorf("%q: %w", t, ErrReservedWildcard)
		}
	}

	// The id and type end up on the wire as SSE fields (and the id in the
	// Last-Event-ID header), so reject all control characters and invalid UTF-8,
	// not only CR/LF/NUL — matching the topic and matcher rules. "#" prefixes are
	// reserved for hub-generated fragment IDs and "earliest" for the reserved
	// last-event-id value; accepting either from a publisher would corrupt
	// reconnection cursors.
	if !validProtocolString(u.ID) ||
		strings.HasPrefix(u.ID, "#") || u.ID == EarliestLastEventID {
		return ErrInvalidEventID
	}

	if !validProtocolString(u.Type) {
		return ErrInvalidEventType
	}

	// "mercure" is reserved for hub-generated events (subscription events set
	// it as the SSE event name); a publisher using it could inject forged
	// events into a client listening for that event type.
	if u.Type == reservedEventType {
		return ErrReservedEventType
	}

	// The protocol requires field values to be valid UTF-8; ParseForm does not
	// enforce it, so reject invalid data rather than dispatch it.
	if !utf8.ValidString(u.Data) {
		return ErrInvalidData
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

	if err := update.Validate(); err != nil {
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
		if err != nil || claims == nil {
			h.httpAuthorizationError(w, r, err)

			if err != nil {
				recordSpanError(span, err)
			}

			return
		}

		// A valid token that carries no publish authorization lacks the scope
		// for this request: 403 insufficient_scope, not 401.
		if claims.Mercure.Publish == nil {
			h.httpInsufficientScopeError(w, r, nil)

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

	// The protocol allows exactly one topic per update; alternate topics are
	// a v8 feature, available only under the deprecated_topic build tag and
	// WithProtocolVersionCompatibility(8).
	if len(topics) > 1 && !h.allowsAlternateTopics() {
		http.Error(w, `Multiple "topic" parameters are not supported anymore, publish one update per topic`, http.StatusBadRequest)

		return
	}

	// Reject oversized topic lists before running canDispatch — otherwise
	// an authenticated publisher could force O(topics × selectors)
	// matching work on every request before being rejected by validate.
	if len(topics) > maxPublishTopics {
		http.Error(w, ErrTooManyTopics.Error(), http.StatusBadRequest)

		return
	}

	// Validate topics before they can reach the shared match cache via
	// canDispatch. Update.Validate() runs later (inside Hub.Publish), but
	// canDispatch keys the cache on the topic list joined with NUL; an
	// unvalidated topic containing a literal NUL would collide with a
	// legitimate multi-topic key and poison the entry (CWE-20).
	for _, t := range topics {
		if !validProtocolString(t) {
			http.Error(w, fmt.Errorf("%q: %w", t, ErrInvalidTopic).Error(), http.StatusBadRequest)

			return
		}
	}

	if claims != nil {
		if err := resolveMatcherClaims(h.topicSelectorStore, claims.Mercure.Publish, h.allowsAlternateTopics()); err != nil {
			writeMatcherClaimError(ctx, h.logger, w, err)
			recordSpanError(span, err)

			return
		}
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
			h.httpInsufficientScopeError(w, r, nil)

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

			h.httpInsufficientScopeError(w, r, nil)

			return
		}
	}

	u = &Update{
		Private: private,
		Debug:   h.debug,
		Event:   Event{r.PostForm.Get("data"), r.PostForm.Get("id"), r.PostForm.Get("type"), retry},
	}
	u.setTopics(topics)

	dispatchCtx := context.WithoutCancel(ctx)

	// Validation, dispatch, logging and metrics live in Hub.Publish.
	if err := h.Publish(dispatchCtx, u); err != nil {
		switch {
		case errors.Is(err, ErrReservedTopic), errors.Is(err, ErrReservedWildcard),
			errors.Is(err, ErrInvalidEventID), errors.Is(err, ErrInvalidEventType),
			errors.Is(err, ErrReservedEventType),
			errors.Is(err, ErrInvalidTopic), errors.Is(err, ErrTooManyTopics),
			errors.Is(err, ErrInvalidData):
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

	// The body is the update id; the protocol requires this exact media type.
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	if _, err := io.WriteString(w, u.ID); err != nil {
		if h.logger.Enabled(ctx, slog.LevelInfo) {
			h.logger.LogAttrs(ctx, slog.LevelInfo, "Failed to write publish response", slog.Any("error", err))
		}

		return
	}
}
