package mercure

import (
	"encoding/json"
	"errors"
	"math/rand/v2"
	"net/http"
	"time"

	"go.uber.org/zap"
)

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

func (rc *responseController) setDispatchWriteDeadline() bool {
	if rc.hub.dispatchTimeout == 0 {
		return true
	}

	deadline := time.Now().Add(rc.hub.dispatchTimeout)
	if deadline.After(rc.writeDeadline) {
		return true
	}

	if err := rc.SetWriteDeadline(deadline); err != nil {
		if c := rc.hub.logger.Check(zap.ErrorLevel, "Unable to set dispatch write deadline"); c != nil {
			c.Write(zap.Object("subscriber", rc.subscriber), zap.Error(err))
		}

		return false
	}

	return true
}

func (rc *responseController) setDefaultWriteDeadline() bool {
	if err := rc.SetWriteDeadline(rc.writeDeadline); err != nil {
		if errors.Is(err, http.ErrNotSupported) {
			panic(err)
		}

		if c := rc.hub.logger.Check(zap.InfoLevel, "Error while setting default write deadline"); c != nil {
			c.Write(zap.Object("subscriber", rc.subscriber), zap.Error(err))
		}

		return false
	}

	return true
}

func (rc *responseController) flush() bool {
	if err := rc.Flush(); err != nil {
		if errors.Is(err, http.ErrNotSupported) {
			panic(err)
		}

		if c := rc.hub.logger.Check(zap.InfoLevel, "Unable to flush"); c != nil {
			c.Write(zap.Object("subscriber", rc.subscriber), zap.Error(err))
		}

		return false
	}

	return true
}

func (h *Hub) newResponseController(w http.ResponseWriter, s *LocalSubscriber) *responseController {
	wd := h.getWriteDeadline(s)

	return &responseController{*http.NewResponseController(w), w, wd.Add(-h.dispatchTimeout), wd, h, s} // nolint:bodyclose
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
	defer h.shutdown(s)

	rc.setDefaultWriteDeadline()

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
			if c := h.logger.Check(zap.DebugLevel, "Connection closed by the client"); c != nil {
				c.Write(zap.Object("subscriber", s))
			}

			return
		case <-heartbeatTimerC:
			// Send an SSE comment as a heartbeat, to prevent issues with some proxies and old browsers
			if !h.write(rc, ":\n") {
				return
			}

			heartbeatTimer.Reset(h.heartbeat)
		case <-disconnectionTimerC:
			// Cleanly close the HTTP connection before the write deadline to prevent client-side errors
			return
		case update, ok := <-s.Receive():
			if !ok || !h.write(rc, newSerializedUpdate(update).event) {
				return
			}

			if heartbeatTimer != nil {
				if !heartbeatTimer.Stop() {
					<-heartbeatTimer.C
				}

				heartbeatTimer.Reset(h.heartbeat)
			}

			if c := h.logger.Check(zap.DebugLevel, "Update sent"); c != nil {
				c.Write(zap.Object("subscriber", s), zap.Object("update", update))
			}
		}
	}
}

// registerSubscriber initializes the connection.
func (h *Hub) registerSubscriber(w http.ResponseWriter, r *http.Request) (*LocalSubscriber, *responseController) { //nolint:funlen
	s := NewLocalSubscriber(retrieveLastEventID(r, h.opt, h.logger), h.logger, h.topicSelectorStore)
	s.RemoteAddr = r.RemoteAddr

	var (
		privateTopics []string
		claims        *claims
	)

	if h.subscriberJWTKeyFunc != nil {
		var err error

		claims, err = authorize(r, h.subscriberJWTKeyFunc, nil, h.cookieName)
		if claims != nil {
			s.Claims = claims
			privateTopics = claims.Mercure.Subscribe
		}

		if err != nil || (claims == nil && !h.anonymous) {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)

			if c := h.logger.Check(zap.DebugLevel, "Subscriber unauthorized"); c != nil {
				c.Write(zap.Object("subscriber", s), zap.Error(err))
			}

			return nil, nil
		}
	}

	topics := r.URL.Query()["topic"]
	if len(topics) == 0 {
		http.Error(w, `Missing "topic" parameter.`, http.StatusBadRequest)

		return nil, nil
	}

	s.SetTopics(topics, privateTopics)

	h.dispatchSubscriptionUpdate(s, true)

	if err := h.transport.AddSubscriber(s); err != nil {
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		h.dispatchSubscriptionUpdate(s, false)

		if c := h.logger.Check(zap.ErrorLevel, "Unable to add subscriber"); c != nil {
			c.Write(zap.Object("subscriber", s), zap.Error(err))
		}

		return nil, nil
	}

	h.sendHeaders(w, s)
	rc := h.newResponseController(w, s)
	rc.flush()

	if c := h.logger.Check(zap.InfoLevel, "New subscriber"); c != nil {
		fields := []LogField{zap.Object("subscriber", s)}
		if claims != nil && h.logger.Level() == zap.DebugLevel {
			fields = append(fields, zap.Reflect("payload", claims.Mercure.Payload))
		}

		c.Write(fields...)
	}

	h.metrics.SubscriberConnected(s)

	return s, rc
}

// sendHeaders sends correct HTTP headers to create a keep-alive connection.
func (h *Hub) sendHeaders(w http.ResponseWriter, s *LocalSubscriber) {
	// Keep alive, useful only for HTTP 1 clients https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Keep-Alive
	w.Header().Set("Connection", "keep-alive")

	// Server-sent events https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events#Sending_events_from_the_server
	w.Header().Set("Content-Type", "text/event-stream")

	// Disable cache, even for old browsers and proxies
	w.Header().Set("Cache-Control", "private, no-cache, no-store, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expire", "0")

	// NGINX support https://www.nginx.com/resources/wiki/start/topics/examples/x-accel/#x-accel-buffering
	w.Header().Set("X-Accel-Buffering", "no")

	if s.RequestLastEventID != "" {
		w.Header().Set("Last-Event-ID", <-s.responseLastEventID)
	}

	// Write a comment in the body
	// Go currently doesn't provide a better way to flush the headers
	if _, err := w.Write([]byte{':', '\n'}); err != nil {
		if c := h.logger.Check(zap.WarnLevel, "Failed to write comment"); c != nil {
			c.Write(zap.Object("subscriber", s), zap.Error(err))
		}
	}
}

// retrieveLastEventID extracts the Last-Event-ID from the corresponding HTTP header with a fallback on the query parameter.
func retrieveLastEventID(r *http.Request, opt *opt, logger Logger) string {
	if id := r.Header.Get("Last-Event-ID"); id != "" {
		return id
	}

	query := r.URL.Query()
	if id := query.Get("lastEventID"); id != "" {
		return id
	}

	if legacyEventIDValues, present := query["Last-Event-ID"]; present {
		if opt.isBackwardCompatiblyEnabledWith(7) {
			logger.Info("Deprecated: the 'Last-Event-ID' query parameter is deprecated since the version 8 of the protocol, use 'lastEventID' instead.")

			if len(legacyEventIDValues) != 0 {
				return legacyEventIDValues[0]
			}
		} else {
			logger.Info("Unsupported: the 'Last-Event-ID' query parameter is not supported anymore, use 'lastEventID' instead or enable backward compatibility with version 7 of the protocol.")
		}
	}

	return ""
}

// Write sends the given string to the client.
// It returns false if the subscriber has been disconnected (e.g. timeout).
func (h *Hub) write(rc *responseController, data string) bool {
	if !rc.setDispatchWriteDeadline() {
		return false
	}

	if _, err := rc.rw.Write([]byte(data)); err != nil {
		if c := h.logger.Check(zap.DebugLevel, "Error writing to client"); c != nil {
			c.Write(zap.Object("subscriber", rc.subscriber), zap.Error(err))
		}

		return false
	}

	return rc.flush() && rc.setDefaultWriteDeadline()
}

func (h *Hub) shutdown(s *LocalSubscriber) {
	// Notify that the client is closing the connection
	s.Disconnect()

	if err := h.transport.RemoveSubscriber(s); err != nil {
		if c := h.logger.Check(zap.WarnLevel, "Failed to remove subscriber on shutdown"); c != nil {
			c.Write(zap.Object("subscriber", s), zap.Error(err))
		}
	}

	h.dispatchSubscriptionUpdate(s, false)

	if c := h.logger.Check(zap.InfoLevel, "Subscriber disconnected"); c != nil {
		c.Write(zap.Object("subscriber", s))
	}

	h.metrics.SubscriberDisconnected(s)
}

func (h *Hub) dispatchSubscriptionUpdate(s *LocalSubscriber, active bool) {
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
		if err := h.transport.Dispatch(u); err != nil {
			if c := h.logger.Check(zap.WarnLevel, "Failed to dispatch update"); c != nil {
				c.Write(zap.Object("subscriber", s), zap.Object("update", u), zap.Error(err))
			}
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
