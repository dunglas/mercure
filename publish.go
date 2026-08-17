package mercure

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
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

// fieldData is the publish form field carrying the update payload.
const fieldData = "data"

// Sentinel errors returned by Publish. Callers can branch on them via
// errors.Is.
var (
	ErrReservedTopic     = errors.New(`topic value resolves into the reserved "/.well-known/mercure" namespace`)
	ErrReservedWildcard  = errors.New(`topic value "*" is reserved for the wildcard matcher and cannot be published`)
	ErrInvalidEventID    = errors.New(`"id" field contains a forbidden control character or invalid UTF-8, starts with "#", or is the reserved value "earliest"`)
	ErrInvalidEventType  = errors.New(`"type" field contains a forbidden control character or invalid UTF-8`)
	ErrReservedEventType = errors.New(`"type" field uses the reserved value "mercure"`)
	ErrInvalidTopic      = errors.New("topic contains a forbidden control character or invalid UTF-8")
	ErrInvalidMediaType  = errors.New("the event content type is not a valid media type")
	ErrTooManyTopics     = errors.New("too many topics in update")
	ErrMissingTopic      = errors.New("update carries no topic")
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
	topics := u.Topics
	if len(topics) == 0 {
		return ErrMissingTopic
	}

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

	// The protocol requires urlencoded field values to be valid UTF-8;
	// ParseForm does not enforce it, so reject invalid data rather than
	// dispatch it. Binary updates (multipart publications) may carry any
	// bytes: they are base64-encoded when serialized to text formats.
	if !u.Binary && !utf8.ValidString(u.Data) {
		return ErrInvalidData
	}

	// The content type ends up on the wire as a header of negotiated response
	// encodings, so a malformed value could inject fields there (CWE-93);
	// ParseMediaType rejects everything but valid media type syntax.
	if u.ContentType != "" {
		if _, _, err := mime.ParseMediaType(u.ContentType); err != nil {
			return fmt.Errorf("%q: %w", u.ContentType, ErrInvalidMediaType)
		}
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

	if h.publisherConfigured {
		var err error

		claims, err = h.authorize(r, true)
		if err != nil || claims == nil {
			h.writeAuthError(w, r, err)

			if err != nil {
				recordSpanError(span, err)
			}

			return
		}
	}

	h.limitRequestBody(w, r)

	var (
		form        url.Values
		contentType string
		binary      bool
	)

	if mediaType, params, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mediaType == "multipart/form-data" { //nolint:nestif
		// Multipart publication is part of the experimental events query
		// support: it exists to carry binary payloads, which only the
		// multipart subscription encoding delivers verbatim.
		if !h.eventsQuery {
			http.Error(w, http.StatusText(http.StatusUnsupportedMediaType), http.StatusUnsupportedMediaType)

			return
		}

		var err error
		if form, contentType, err = parseMultipartPublication(r.Body, params["boundary"]); err != nil {
			writeParseError(w, err)

			return
		}

		binary = true
	} else if err := r.ParseForm(); err != nil {
		writeParseError(w, err)

		return
	} else {
		form = r.PostForm
	}

	topics := form["topic"]
	if len(topics) == 0 {
		http.Error(w, `Missing "topic" parameter`, http.StatusBadRequest)

		return
	}

	// Reject oversized topic lists before running canDispatch — otherwise
	// an authenticated publisher could force O(topics × matchers)
	// matching work on every request before being rejected by validate.
	if len(topics) > maxPublishTopics {
		http.Error(w, ErrTooManyTopics.Error(), http.StatusBadRequest)

		return
	}

	// Validate topics before they can reach the shared match cache via the
	// authorization grant check (grantsAll → matches → cachedMatch), which
	// keys the cache on the topic list joined with NUL; an unvalidated topic
	// containing a literal NUL would collide with a legitimate multi-topic key
	// and poison the entry (CWE-20). Update.Validate() re-checks later, but only
	// after the grant check has already consulted the cache.
	for _, t := range topics {
		if !validProtocolString(t) {
			http.Error(w, fmt.Errorf("%q: %w", t, ErrInvalidTopic).Error(), http.StatusBadRequest)

			return
		}
	}

	var retry uint64

	if retryString := form.Get("retry"); retryString != "" {
		var err error
		if retry, err = strconv.ParseUint(retryString, 10, 64); err != nil {
			http.Error(w, `Invalid "retry" parameter`, http.StatusBadRequest)

			return
		}
	}

	private := len(form["private"]) != 0
	if claims != nil && !claims.authz.grantsAll(h.topicMatcherStore, actionPublish, topics) { //nolint:nestif
		if private {
			h.writeBearerError(w, r, bearerErrInsufficientScope, http.StatusForbidden)

			return
		}

		infoEnabled := h.logger.Enabled(ctx, slog.LevelInfo)
		if h.isBackwardCompatiblyEnabledWith(7) {
			if infoEnabled {
				h.logger.LogAttrs(ctx, slog.LevelInfo, `Deprecated: posting public updates to topics not granted to the token is deprecated since the version 7 of the protocol, grant the "*" topic to allow publishing on all topics.`)
			}
		} else {
			if infoEnabled {
				h.logger.LogAttrs(ctx, slog.LevelInfo, `Unsupported: posting public updates to topics not granted to the token is not supported anymore, grant the "*" topic to allow publishing on all topics or enable backward compatibility with the version 7 of the protocol.`)
			}

			h.writeBearerError(w, r, bearerErrInsufficientScope, http.StatusForbidden)

			return
		}
	}

	u = &Update{
		Topics:      topics,
		Private:     private,
		Debug:       h.debug,
		ContentType: contentType,
		Binary:      binary,
		Event:       Event{form.Get(fieldData), form.Get("id"), form.Get("type"), retry},
	}

	dispatchCtx := context.WithoutCancel(ctx)

	// Validation, dispatch, logging and metrics live in Hub.Publish.
	if err := h.Publish(dispatchCtx, u); err != nil {
		switch {
		case errors.Is(err, ErrReservedTopic), errors.Is(err, ErrReservedWildcard),
			errors.Is(err, ErrInvalidEventID), errors.Is(err, ErrInvalidEventType),
			errors.Is(err, ErrReservedEventType),
			errors.Is(err, ErrInvalidTopic), errors.Is(err, ErrTooManyTopics),
			errors.Is(err, ErrMissingTopic), errors.Is(err, ErrInvalidData),
			errors.Is(err, ErrInvalidMediaType):
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

// writeParseError maps a publication body parsing failure to its status:
// 413 when the MaxBytesReader cap fired, 400 otherwise.
func writeParseError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest

	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		status = http.StatusRequestEntityTooLarge
	}

	http.Error(w, http.StatusText(status), status)
}

// parseMultipartPublication reads a multipart/form-data publication body.
// Fields parse exactly like their urlencoded counterparts, but the data
// part keeps its raw bytes — the reason this encoding exists: urlencoded
// field values must be UTF-8, so they cannot carry binary payloads — and
// its explicit Content-Type header, if any, declares the event media type.
// The caller has already capped the body with MaxBytesReader.
func parseMultipartPublication(body io.Reader, boundary string) (url.Values, string, error) {
	if boundary == "" {
		return nil, "", http.ErrMissingBoundary
	}

	var contentType string

	form := url.Values{}
	mr := multipart.NewReader(body, boundary)

	for {
		p, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			return form, contentType, nil
		}

		if err != nil {
			return nil, "", fmt.Errorf("invalid multipart publication body: %w", err)
		}

		name := p.FormName()
		if name == "" {
			// A part without a field name has no urlencoded equivalent;
			// ignore it like an unknown field.
			continue
		}

		v, err := io.ReadAll(p)
		if err != nil {
			return nil, "", fmt.Errorf("unable to read multipart publication part: %w", err)
		}

		if name == fieldData {
			contentType = p.Header.Get("Content-Type")
		}

		form.Add(name, string(v))
	}
}
