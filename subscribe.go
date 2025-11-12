package mercure

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"time"
)

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

	if err := rc.SetWriteDeadline(deadline); err != nil {
		rc.hub.logger.ErrorContext(ctx, "Unable to set dispatch write deadline", slog.Any("error", err))

		return false
	}

	return true
}

func (rc *responseController) setDefaultWriteDeadline(ctx context.Context) bool {
	if err := rc.SetWriteDeadline(rc.writeDeadline); err != nil {
		if errors.Is(err, http.ErrNotSupported) {
			panic(err)
		}

		rc.hub.logger.InfoContext(ctx, "Error while setting default write deadline", slog.Any("error", err))

		return false
	}

	return true
}

func (rc *responseController) flush(ctx context.Context) bool {
	if err := rc.Flush(); err != nil {
		if errors.Is(err, http.ErrNotSupported) {
			panic(err)
		}

		rc.hub.logger.InfoContext(ctx, "Unable to flush", slog.Any("error", err))

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
	s, rc := h.registerSubscriber(w, r)
	if s == nil {
		return
	}

	ctx := context.WithValue(r.Context(), SubscriberContextKey, &s.Subscriber)

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

	if h.writeTimeout != 0 {
		disconnectionTimer := time.NewTimer(time.Until(rc.disconnectionTime))
		defer disconnectionTimer.Stop()

		disconnectionTimerC = disconnectionTimer.C
	}

	for {
		select {
		case <-r.Context().Done():
			rc.hub.logger.DebugContext(ctx, "Connection closed by the client")

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

			rc.hub.logger.DebugContext(ctx, "Update sent", slog.Any("update", update))
		}
	}
}

// registerSubscriber initializes the connection.
func (h *Hub) registerSubscriber(w http.ResponseWriter, r *http.Request) (*LocalSubscriber, *responseController) { //nolint:funlen
	s := NewLocalSubscriber(h.retrieveLastEventID(r), h.logger, h.topicSelectorStore)
	ctx := r.Context()

	var (
		privateTopics []string
		claims        *claims
	)

	if h.subscriberJWTKeyFunc != nil {
		var err error

		claims, err = h.authorize(r, false)
		if claims != nil {
			s.Claims = claims
			privateTopics = claims.Mercure.Subscribe
		}

		if err != nil || (claims == nil && !h.anonymous) {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)

			h.logger.DebugContext(ctx, "Subscriber unauthorized", slog.Any("error", err))

			return nil, nil
		}
	}

	topics := r.URL.Query()["topic"]
	if len(topics) == 0 {
		http.Error(w, `Missing "topic" parameter.`, http.StatusBadRequest)

		return nil, nil
	}

	s.SetTopics(topics, privateTopics)

	h.dispatchSubscriptionUpdate(ctx, s, true)

	if err := h.transport.AddSubscriber(ctx, s); err != nil {
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		h.dispatchSubscriptionUpdate(ctx, s, false)

		h.logger.ErrorContext(ctx, "Unable to add subscriber", slog.Any("error", err))

		return nil, nil
	}

	h.sendHeaders(ctx, w, s)
	rc := h.newResponseController(w, s)
	rc.flush(r.Context())

	if h.logger.Enabled(ctx, slog.LevelInfo) {
		if claims != nil && h.logger.Enabled(ctx, slog.LevelDebug) {
			h.logger.InfoContext(ctx, "New subscriber", slog.Any("payload", claims.Mercure.Payload))
		}
	} else {
		h.logger.InfoContext(ctx, "New subscriber")
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
		header["Last-Event-ID"] = []string{<-s.responseLastEventID}
	}

	// Write a comment in the body
	// Go currently doesn't provide a better way to flush the headers
	if _, err := w.Write([]byte{':', '\n'}); err != nil {
		h.logger.InfoContext(ctx, "Failed to write comment", slog.Any("error", err))
	}
}

// retrieveLastEventID extracts the Last-Event-ID from the corresponding HTTP header with a fallback on the query parameter.
func (h *Hub) retrieveLastEventID(r *http.Request) string {
	if id := r.Header.Get("Last-Event-ID"); id != "" {
		return id
	}

	query := r.URL.Query()
	if id := query.Get("lastEventID"); id != "" {
		return id
	}

	if legacyEventIDValues, present := query["Last-Event-ID"]; present {
		if h.isBackwardCompatiblyEnabledWith(7) {
			h.logger.Info("Deprecated: the 'Last-Event-ID' query parameter is deprecated since the version 8 of the protocol, use 'lastEventID' instead.")

			if len(legacyEventIDValues) != 0 {
				return legacyEventIDValues[0]
			}
		} else {
			h.logger.Info(`Unsupported: the "Last-Event-ID"" query parameter is not supported anymore, use "lastEventID"" instead or enable backward compatibility with version 7 of the protocol.`)
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

	if _, err := rc.rw.Write([]byte(data)); err != nil {
		h.logger.DebugContext(ctx, "Failed to write comment", slog.Any("error", err))

		return false
	}

	return rc.flush(ctx) && rc.setDefaultWriteDeadline(ctx)
}

func (h *Hub) shutdown(ctx context.Context, s *LocalSubscriber) {
	// Notify that the client is closing the connection
	s.Disconnect()

	if err := h.transport.RemoveSubscriber(ctx, s); err != nil {
		h.logger.WarnContext(ctx, "Failed to remove subscriber on shutdown", slog.Any("error", err))
	}

	h.dispatchSubscriptionUpdate(ctx, s, false)
	h.logger.InfoContext(ctx, "Subscriber disconnected")
	h.metrics.SubscriberDisconnected(s)
}

func (h *Hub) dispatchSubscriptionUpdate(ctx context.Context, s *LocalSubscriber, active bool) {
	if !h.subscriptions {
		return
	}

	for _, subscription := range s.getSubscriptions("", jsonldContext, active) {
		j, err := json.MarshalIndent(subscription, "", "  ")
		if err != nil {
			panic(err)
		}

		u := &Update{
			Topics:  []string{subscription.ID},
			Private: true,
			Debug:   h.debug,
			Event:   Event{Data: string(j)},
		}

		if err := h.transport.Dispatch(ctx, u); err != nil {
			h.logger.WarnContext(ctx, "Failed to dispatch update", slog.Any("update", u), slog.Any("subscription", subscription.ID), slog.Any("error", err))
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
