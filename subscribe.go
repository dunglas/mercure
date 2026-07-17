package mercure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/url"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// methodQuery is the safe, idempotent HTTP QUERY method (RFC 9110 semantics,
// defined in RFC 10008). Subscribers use it to send the topic matcher list in
// the request body instead of the URL, avoiding query-string length limits.
const methodQuery = "QUERY"

type subscriberContextKeyType struct{}

var SubscriberContextKey subscriberContextKeyType //nolint:gochecknoglobals

type responseController struct {
	http.ResponseController

	rw http.ResponseWriter

	// disconnectionTime is the JWT expiration date minus hub.dispatchTimeout, or time.Now() plus hub.writeTimeout minus hub.dispatchTimeout
	disconnectionTime time.Time
	// writeDeadline is the JWT expiration date or time.Now() + hub.writeTimeout
	writeDeadline time.Time
	hub           *Hub
	subscriber    *LocalSubscriber
}

func (rc *responseController) setDispatchWriteDeadline(ctx context.Context) bool {
	if rc.hub.dispatchTimeout == 0 {
		return true
	}

	deadline := time.Now().Add(rc.hub.dispatchTimeout)
	if deadline.After(rc.writeDeadline) {
		return true
	}

	if err := rc.SetWriteDeadline(deadline); err != nil && rc.hub.logger.Enabled(ctx, slog.LevelInfo) {
		rc.hub.logger.LogAttrs(ctx, slog.LevelInfo, "Unable to set dispatch write deadline", slog.Any("error", err))

		return false
	}

	return true
}

func (rc *responseController) setDefaultWriteDeadline(ctx context.Context) bool {
	if err := rc.SetWriteDeadline(rc.writeDeadline); err != nil {
		rc.hub.handleWriterError(ctx, err, "Error while setting default write deadline")

		return false
	}

	return true
}

func (rc *responseController) flush(ctx context.Context) bool {
	if err := rc.Flush(); err != nil {
		rc.hub.handleWriterError(ctx, err, "Error while flushing response")

		return false
	}

	return true
}

func (h *Hub) newResponseController(w http.ResponseWriter, s *LocalSubscriber) *responseController {
	wd := h.getWriteDeadline(s)

	return &responseController{
		*http.NewResponseController(w), // nolint:bodyclose
		w,
		wd.Add(-h.dispatchTimeout),
		wd,
		h,
		s,
	}
}

func (h *Hub) getWriteDeadline(s *LocalSubscriber) (deadline time.Time) {
	if h.writeTimeout != 0 {
		deadline = time.Now().Add(randomizeWriteDeadline(h.writeTimeout))
	}

	if s.Claims != nil && s.Claims.ExpiresAt != nil && (deadline.Equal(time.Time{}) || s.Claims.ExpiresAt.Before(deadline)) {
		now := time.Now()
		deadline = now.Add(randomizeWriteDeadline(s.Claims.ExpiresAt.Sub(now)))
	}

	return deadline
}

// SubscribeHandler creates a keep alive connection and sends the events to the subscribers.
//
//nolint:funlen,gocognit
func (h *Hub) SubscribeHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	s, rc := h.registerSubscriber(ctx, w, r)
	if s == nil {
		return
	}

	ctx = context.WithValue(ctx, SubscriberContextKey, &s.Subscriber)

	defer h.shutdown(ctx, s)

	rc.setDefaultWriteDeadline(ctx)

	var (
		heartbeatTimer      *time.Timer
		heartbeatTimerC     <-chan time.Time
		disconnectionTimerC <-chan time.Time
	)

	if h.heartbeat != 0 {
		heartbeatTimer = time.NewTimer(h.heartbeat)
		defer heartbeatTimer.Stop()

		heartbeatTimerC = heartbeatTimer.C
	}

	// Arm the disconnection timer whenever a write deadline exists, including
	// when it comes solely from the token's exp (write_timeout disabled):
	// getWriteDeadline leaves the deadline zero only when neither a write
	// timeout nor a token exp applies. The protocol requires closing the
	// connection no later than exp, so relying on a failed write against a past
	// deadline would otherwise leave an authenticated connection open up to a
	// heartbeat interval past exp, or indefinitely with heartbeat off.
	if !rc.writeDeadline.IsZero() {
		disconnectionTimer := time.NewTimer(time.Until(rc.disconnectionTime))
		defer disconnectionTimer.Stop()

		disconnectionTimerC = disconnectionTimer.C
	}

	debugLevel := rc.hub.logger.Enabled(ctx, slog.LevelDebug)

	// On hub shutdown (Caddy "stopping" event, pod SIGTERM, …) we prefer to
	// let each subscriber drain on its own per-connection write deadline
	// (derived from writeTimeout, and optionally shortened by JWT expiry)
	// rather than closing everything at once — that spreads the reconnect
	// load at the same pace clients already experience in steady state,
	// instead of producing a synchronized storm on the ingress and the
	// transport. The orchestrator's grace period (k8s
	// terminationGracePeriodSeconds, etc.) remains the hard deadline.
	//
	// When writeTimeout is disabled (0) there is no disconnectionTimerC, so
	// the only way out on shutdown is still h.ctx.Done() — otherwise
	// http.Server.Shutdown would hang indefinitely on active handlers.
	var hubCtxDoneC <-chan struct{}
	if h.writeTimeout == 0 {
		hubCtxDoneC = h.ctx.Done()
	}

	for {
		select {
		case <-hubCtxDoneC:
			if debugLevel {
				rc.hub.logger.LogAttrs(ctx, slog.LevelDebug, "Hub is shutting down, closing connection")
			}

			return
		case <-ctx.Done():
			if debugLevel {
				rc.hub.logger.LogAttrs(ctx, slog.LevelDebug, "Connection closed by the client")
			}

			return
		case <-heartbeatTimerC:
			// Send an SSE comment as a heartbeat, to prevent issues with some proxies and old browsers
			if !h.write(ctx, rc, ":\n") {
				return
			}

			heartbeatTimer.Reset(h.heartbeat)
		case <-disconnectionTimerC:
			// Cleanly close the HTTP connection before the write deadline to prevent client-side errors
			return
		case update, ok := <-s.Receive():
			if !ok || !h.write(ctx, rc, newSerializedUpdate(update).event) {
				return
			}

			if heartbeatTimer != nil {
				if !heartbeatTimer.Stop() {
					<-heartbeatTimer.C
				}

				heartbeatTimer.Reset(h.heartbeat)
			}

			if debugLevel {
				rc.hub.logger.LogAttrs(ctx, slog.LevelDebug, "Update sent", slog.Any("update", update))
			}
		}
	}
}

// registerSubscriber initializes the connection.
func (h *Hub) registerSubscriber(ctx context.Context, w http.ResponseWriter, r *http.Request) (*LocalSubscriber, *responseController) { //nolint:funlen
	ctx, span := startSpan(ctx, "mercure.subscribe", trace.WithSpanKind(trace.SpanKindConsumer))
	defer span.End()

	values, err := h.subscribeValues(r)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		recordSpanError(span, err)

		return nil, nil
	}

	s := NewLocalSubscriber(h.retrieveLastEventID(ctx, r, values), h.logger, h.topicMatcherStore)

	var claims *claims

	if h.subscriberJWTKeyFunc != nil { //nolint:nestif
		var err error

		claims, err = h.authorize(r, false)
		if claims != nil {
			s.Claims = claims
		}

		if err != nil || (claims == nil && !h.anonymous) {
			h.writeAuthError(w, r, err)

			if err != nil {
				recordSpanError(span, err)
			}

			return nil, nil
		}
	}

	deprecated := h.isBackwardCompatiblyEnabledWith(8)

	matchers, err := h.parseMatchers(values, deprecated)
	if err != nil {
		h.writeMatcherParamError(ctx, w, err)
		recordSpanError(span, err)

		return nil, nil
	}

	var privateTopicMatchers []TopicMatcher
	if claims != nil {
		privateTopicMatchers = claims.authz.subscribeMatchers()
	}

	s.setMatchers(matchers, privateTopicMatchers)

	if span.IsRecording() {
		span.SetAttributes(
			attribute.String("mercure.subscriber.id", s.ID),
			attribute.StringSlice("mercure.topics", logMatcherPatterns(matchers)),
		)
	}

	addCtx := context.WithoutCancel(ctx)
	h.dispatchSubscriptionUpdate(addCtx, s, true)

	if err := h.transport.AddSubscriber(addCtx, s); err != nil {
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		h.dispatchSubscriptionUpdate(addCtx, s, false)

		if h.logger.Enabled(ctx, slog.LevelError) {
			h.logger.LogAttrs(ctx, slog.LevelError, "Unable to add subscriber", slog.Any("error", err))
		}

		recordSpanError(span, err)

		return nil, nil
	}

	h.sendHeaders(ctx, w, s)
	rc := h.newResponseController(w, s)
	rc.flush(ctx)

	if h.logger.Enabled(ctx, slog.LevelInfo) {
		if claims != nil && h.logger.Enabled(ctx, slog.LevelDebug) {
			h.logger.LogAttrs(ctx, slog.LevelInfo, "New subscriber", slog.Any("payload", s.SubscriptionPayloads))
		} else {
			h.logger.LogAttrs(ctx, slog.LevelInfo, "New subscriber")
		}
	}

	h.metrics.SubscriberConnected(s)

	return s, rc
}

//nolint:gochecknoglobals
var (
	headerConnection   = []string{"keep-alive"}
	headerContentType  = []string{"text/event-stream"}
	headerCacheControl = []string{"private, no-cache, no-store, must-revalidate, max-age=0"}
	headerPragma       = []string{"no-cache"}
	headerExpire       = []string{"0"}

	headerXAccelBuffering = []string{"no"}
)

// sendHeaders sends correct HTTP headers to create a keep-alive connection.
func (h *Hub) sendHeaders(ctx context.Context, w http.ResponseWriter, s *LocalSubscriber) {
	header := w.Header()

	// Keep alive, useful only for HTTP 1 clients https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Keep-Alive
	header["Connection"] = headerConnection

	header["Content-Type"] = headerContentType

	// Disable cache, even for old browsers and proxies
	header["Cache-Control"] = headerCacheControl
	header["Pragma"] = headerPragma
	header["Expire"] = headerExpire

	// NGINX support https://www.nginx.com/resources/wiki/start/topics/examples/x-accel/#x-accel-buffering
	header["X-Accel-Buffering"] = headerXAccelBuffering

	if s.RequestLastEventID != "" {
		header["Mercure-Last-Event-Id"] = []string{<-s.responseLastEventID}
	}

	// Write a comment in the body
	// Go currently doesn't provide a better way to flush the headers
	if _, err := w.Write([]byte{':', '\n'}); err != nil && h.logger.Enabled(ctx, slog.LevelInfo) {
		h.logger.LogAttrs(ctx, slog.LevelInfo, "Failed to write comment", slog.Any("error", err))
	}
}

// subscribeValues returns the subscription parameters. For GET and HEAD they
// come from the URL query; for QUERY the application/x-www-form-urlencoded
// request body is parsed and merged on top, so a subscriber can pass topics
// either way. Body size is bounded upstream (e.g. Caddy's request_body
// directive), as for the publish endpoint.
func (h *Hub) subscribeValues(r *http.Request) (url.Values, error) {
	values := r.URL.Query()
	if r.Method != methodQuery {
		return values, nil
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("reading QUERY request body: %w", err)
	}

	bodyValues, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, fmt.Errorf("parsing QUERY request body: %w", err)
	}

	for k, vs := range bodyValues {
		values[k] = append(values[k], vs...)
	}

	return values, nil
}

// retrieveLastEventID extracts the Last-Event-ID from the corresponding HTTP header with a fallback on the query parameter.
func (h *Hub) retrieveLastEventID(ctx context.Context, r *http.Request, query url.Values) string {
	if id := r.Header.Get("Last-Event-ID"); id != "" {
		return id
	}

	if id := query.Get("last_event_id"); id != "" {
		return id
	}

	if legacyEventIDValues, present := query["Last-Event-ID"]; present { //nolint:nestif
		infoLevel := h.logger.Enabled(ctx, slog.LevelInfo)
		if h.isBackwardCompatiblyEnabledWith(7) {
			if infoLevel {
				h.logger.LogAttrs(ctx, slog.LevelInfo, "Deprecated: the 'Last-Event-ID' query parameter is deprecated since the version 8 of the protocol, use 'last_event_id' instead.")
			}

			if len(legacyEventIDValues) != 0 {
				return legacyEventIDValues[0]
			}
		} else if infoLevel {
			h.logger.LogAttrs(ctx, slog.LevelInfo, `Unsupported: the "Last-Event-ID" query parameter is not supported anymore, use "last_event_id" instead or enable backward compatibility with version 7 of the protocol.`)
		}
	}

	return ""
}

// Write sends the given string to the client.
// It returns false if the subscriber has been disconnected (e.g. timeout).
func (h *Hub) write(ctx context.Context, rc *responseController, data string) bool {
	if !rc.setDispatchWriteDeadline(ctx) {
		return false
	}

	if _, err := rc.rw.Write([]byte(data)); err != nil && h.logger.Enabled(ctx, slog.LevelDebug) {
		h.logger.LogAttrs(ctx, slog.LevelDebug, "Failed to write comment", slog.Any("error", err))

		return false
	}

	return rc.flush(ctx) && rc.setDefaultWriteDeadline(ctx)
}

func (h *Hub) shutdown(ctx context.Context, s *LocalSubscriber) {
	// Notify that the client is closing the connection
	s.Disconnect()

	ctx = context.WithoutCancel(ctx)

	if err := h.transport.RemoveSubscriber(ctx, s); err != nil && h.logger.Enabled(ctx, slog.LevelError) {
		h.logger.LogAttrs(ctx, slog.LevelError, "Failed to remove subscriber on shutdown", slog.Any("error", err))
	}

	h.dispatchSubscriptionUpdate(ctx, s, false)

	if h.logger.Enabled(ctx, slog.LevelInfo) {
		h.logger.LogAttrs(ctx, slog.LevelInfo, "Subscriber disconnected")
	}

	h.metrics.SubscriberDisconnected(s)
}

func (h *Hub) dispatchSubscriptionUpdate(ctx context.Context, s *LocalSubscriber, active bool) {
	if !h.subscriptions {
		return
	}

	for _, subscription := range s.getSubscriptions(subscriptionFilter{}, active) {
		j, err := json.MarshalIndent(subscription, "", "  ")
		if err != nil {
			panic(err)
		}

		// Dispatched directly, bypassing Hub.Publish/Update.Validate: this is
		// the only path allowed to set the reserved reservedEventType, and
		// Validate would reject it. Safe because Topic and Data are hub-built
		// here (subscription.ID is a hub-constructed path; json.MarshalIndent
		// escapes control characters), not attacker-controlled. Keep that
		// invariant if this function changes.
		u := &Update{
			Topic:   subscription.ID,
			Private: true,
			Debug:   h.debug,
			Event:   Event{Data: string(j), Type: reservedEventType},
		}

		if err := h.transport.Dispatch(ctx, u); err != nil && h.logger.Enabled(ctx, slog.LevelError) {
			h.logger.LogAttrs(ctx, slog.LevelError, "Failed to dispatch update", slog.Any("update", u), slog.Any("subscription", subscription.ID), slog.Any("error", err))
		}
	}
}

// randomizeWriteDeadline generates a random duration between 80% and 100% of the original value.
// This is useful to avoid all subscribers disconnecting at the same time, which can lead to a thundering herd problem.
func randomizeWriteDeadline(originalValue time.Duration) time.Duration {
	minV := int64(float64(originalValue) * 0.80)
	maxV := int64(originalValue)

	// Ensure min is not greater than max. This handles cases where originalValue is very small (e.g., 1, 2, 3, 4).
	// For originalValue = 1, min becomes 0. For originalValue = 4, min becomes 3.
	// This shouldn't happen in practice, but it's a good safeguard.
	if minV > maxV {
		minV = maxV
	}

	// Calculate the range size. Add 1 because Int64N is exclusive of the upper bound.
	rangeSize := maxV - minV + 1

	// If rangeSize is 0 or less (e.g., if originalValue was 0), just return min (which would be 0).
	// rand.Int64N requires a positive argument.
	if rangeSize <= 0 {
		return time.Duration(minV)
	}

	// Generate a random number in the range [min, max]
	// rand.Int64n(n) returns a non-negative pseudo-random 64-bit integer in the half-open interval [0, n).
	// Adding 'min' shifts this result to the desired range [min, max].
	return time.Duration(rand.Int64N(rangeSize) + minV) //nolint:gosec
}

func (h *Hub) handleWriterError(ctx context.Context, err error, message string) {
	if errors.Is(err, http.ErrNotSupported) {
		panic(err)
	}

	if h.logger.Enabled(ctx, slog.LevelInfo) {
		h.logger.LogAttrs(ctx, slog.LevelInfo, message, slog.Any("error", err))
	}
}

// writeMatcherParamError answers a subscribe-query matcher error with 400. For
// an invalid pattern it writes a generic message and logs the detail: the
// underlying URL Pattern compiler can embed internal memory addresses in its
// error text (CWE-209), which must not reach the client.
func (h *Hub) writeMatcherParamError(ctx context.Context, w http.ResponseWriter, err error) {
	if errors.Is(err, errInvalidMatcherPattern) {
		http.Error(w, errInvalidMatcherPattern.Error(), http.StatusBadRequest)

		if h.logger.Enabled(ctx, slog.LevelDebug) {
			h.logger.LogAttrs(ctx, slog.LevelDebug, "Invalid topic matcher pattern in subscribe request", slog.Any("error", err))
		}

		return
	}

	http.Error(w, err.Error(), http.StatusBadRequest)
}
