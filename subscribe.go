package mercure

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math/rand/v2"
	"net/http"
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

	// Disconnect one dispatch before the write deadline so the client sees a
	// clean end of stream instead of a failed write. That subtraction lands in
	// the past when the deadline is nearer than dispatchTimeout — a token
	// expiring within it, or a dispatchTimeout larger than writeTimeout — which
	// would close the connection as soon as it opened and put the subscriber in
	// a reconnect loop. Fall back to the deadline itself: less margin, but the
	// subscriber gets the time its token grants. A zero deadline means no
	// deadline at all, and SubscribeHandler then arms no timer.
	dt := wd
	if !wd.IsZero() {
		if d := wd.Add(-h.dispatchTimeout); d.After(time.Now()) {
			dt = d
		}
	}

	return &responseController{
		*http.NewResponseController(w), // nolint:bodyclose
		w,
		dt,
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

	s, rc, enc := h.registerSubscriber(ctx, w, r)
	if s == nil {
		return
	}

	// The response begins here, once the subscriber is registered. Registering
	// one and answering it are separate concerns, and only the second belongs
	// to a handler that then keeps the connection.
	h.sendHeaders(ctx, w, s, enc.contentType(), enc.preamble())
	rc.flush(ctx)

	ctx = context.WithValue(ctx, SubscriberContextKey, &s.Subscriber)

	defer h.shutdown(ctx, s)

	rc.setDefaultWriteDeadline(ctx)

	var (
		heartbeatTimer      *time.Timer
		heartbeatTimerC     <-chan time.Time
		disconnectionTimerC <-chan time.Time
	)

	heartbeat := enc.heartbeat()
	if h.heartbeat != 0 && heartbeat != "" {
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
			// Keep the connection alive, to prevent issues with some proxies and old browsers
			if !h.write(ctx, rc, enc.heartbeat()) {
				return
			}

			heartbeatTimer.Reset(h.heartbeat)
		case <-disconnectionTimerC:
			// Cleanly close the HTTP connection before the write deadline to prevent client-side errors
			return
		case update, ok := <-s.Receive():
			if !ok || !h.write(ctx, rc, enc.encode(update)) {
				return
			}

			// An update counts as activity, so push the heartbeat back. Go 1.23
			// made timer channels unbuffered and has Reset discard any pending
			// value, so Reset alone is enough: no Stop-and-drain dance.
			if heartbeatTimer != nil {
				heartbeatTimer.Reset(h.heartbeat)
			}

			if debugLevel {
				rc.hub.logger.LogAttrs(ctx, slog.LevelDebug, "Update sent", slog.Any("update", update))
			}
		}
	}
}

// registerSubscriber initializes the connection.
func (h *Hub) registerSubscriber(ctx context.Context, w http.ResponseWriter, r *http.Request) (*LocalSubscriber, *responseController, streamEncoder) { //nolint:funlen
	ctx, span := startSpan(ctx, "mercure.subscribe", trace.WithSpanKind(trace.SpanKindConsumer))
	defer span.End()

	h.limitRequestBody(w, r)

	req, parseErr := h.parseSubscribeRequest(ctx, r)
	if parseErr != nil {
		http.Error(w, http.StatusText(parseErr.status), parseErr.status)
		recordSpanError(span, parseErr.cause)

		return nil, nil, nil
	}

	s := NewLocalSubscriber(req.lastEventID, h.logger, h.topicMatcherStore)
	s.RequestLastEventIDSet = req.lastEventIDSet

	var claims *claims

	if h.subscriberConfigured { //nolint:nestif
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

			return nil, nil, nil
		}
	}

	deprecated := h.isBackwardCompatiblyEnabledWith(8)

	matchers, err := h.parseMatchers(req.values, deprecated)
	if err != nil {
		h.writeMatcherParamError(ctx, w, err)
		recordSpanError(span, err)

		return nil, nil, nil
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

	if err := h.transport.AddSubscriber(addCtx, s); err != nil {
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)

		if h.logger.Enabled(ctx, slog.LevelError) {
			h.logger.LogAttrs(ctx, slog.LevelError, "Unable to add subscriber", slog.Any("error", err))
		}

		recordSpanError(span, err)

		return nil, nil, nil
	}

	// Announce the subscription only once it exists, so a failed registration
	// cannot publish an active:true for a subscriber that never connected and
	// then have to take it back. shutdown() already announces termination in
	// this order: remove first, then dispatch active:false.
	h.dispatchSubscriptionUpdate(addCtx, s, true)

	rc := h.newResponseController(w, s)
	enc := eventStreamEncoder{}

	if h.logger.Enabled(ctx, slog.LevelInfo) {
		if claims != nil && h.logger.Enabled(ctx, slog.LevelDebug) {
			h.logger.LogAttrs(ctx, slog.LevelInfo, "New subscriber", slog.Any("payload", s.SubscriptionPayloads))
		} else {
			h.logger.LogAttrs(ctx, slog.LevelInfo, "New subscriber")
		}
	}

	h.metrics.SubscriberConnected(s)

	return s, rc, enc
}

//nolint:gochecknoglobals
var (
	headerConnection   = []string{"keep-alive"}
	headerCacheControl = []string{"private, no-cache, no-store, must-revalidate, max-age=0"}
	headerPragma       = []string{"no-cache"}
	headerExpire       = []string{"0"}

	headerXAccelBuffering = []string{"no"}
)

// sendHeaders sends correct HTTP headers to create a keep-alive connection.
func (h *Hub) sendHeaders(ctx context.Context, w http.ResponseWriter, s *LocalSubscriber,
	contentType []string, preamble string,
) {
	header := w.Header()

	// Keep alive, useful only for HTTP 1 clients https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Keep-Alive
	header["Connection"] = headerConnection

	header["Content-Type"] = contentType

	// Disable cache, even for old browsers and proxies
	header["Cache-Control"] = headerCacheControl
	header["Pragma"] = headerPragma
	header["Expire"] = headerExpire

	// NGINX support https://www.nginx.com/resources/wiki/start/topics/examples/x-accel/#x-accel-buffering
	header["X-Accel-Buffering"] = headerXAccelBuffering

	if s.RequestLastEventIDSet {
		header["Mercure-Last-Event-Id"] = []string{<-s.responseLastEventID}
	}

	// Write the framing's preamble in the body
	// Go currently doesn't provide a better way to flush the headers
	if _, err := w.Write([]byte(preamble)); err != nil && h.logger.Enabled(ctx, slog.LevelInfo) {
		h.logger.LogAttrs(ctx, slog.LevelInfo, "Failed to write preamble", slog.Any("error", err))
	}
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
			Topics:  []string{subscription.ID},
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
