package mercure

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// SubscribeHandler creates a keep alive connection and sends the events to the subscribers.
func (h *Hub) SubscribeHandler(w http.ResponseWriter, r *http.Request) {
	assertFlusher(w)

	s := h.registerSubscriber(w, r)
	if s == nil {
		return
	}
	defer h.shutdown(s)

	var heartbeatTimer *time.Timer
	var heartbeatTimerC <-chan time.Time
	if h.heartbeat != 0 {
		heartbeatTimer = time.NewTimer(h.heartbeat)
		defer heartbeatTimer.Stop()
		heartbeatTimerC = heartbeatTimer.C
	}

	var writeTimer *time.Timer
	var writeTimerC <-chan time.Time
	if h.writeTimeout != 0 {
		writeTimer = time.NewTimer(h.writeTimeout - h.dispatchTimeout)
		defer writeTimer.Stop()
		writeTimerC = writeTimer.C
	}

	for {
		select {
		case <-r.Context().Done():
			if c := h.logger.Check(zap.DebugLevel, "connection closed by the client"); c != nil {
				c.Write(zap.Object("subscriber", s))
			}

			return
		case <-writeTimerC:
			if c := h.logger.Check(zap.DebugLevel, "write timeout: close the connection"); c != nil {
				c.Write(zap.Object("subscriber", s))
			}

			return
		case <-heartbeatTimerC:
			// Send a SSE comment as a heartbeat, to prevent issues with some proxies and old browsers
			if !h.write(w, s, ":\n") {
				return
			}
			heartbeatTimer.Reset(h.heartbeat)
		case update, ok := <-s.Receive():
			if !ok || !h.write(w, s, newSerializedUpdate(update).event) {
				return
			}
			if heartbeatTimer != nil {
				if !heartbeatTimer.Stop() {
					<-heartbeatTimer.C
				}
				heartbeatTimer.Reset(h.heartbeat)
			}
			if c := h.logger.Check(zap.InfoLevel, "Update sent"); c != nil {
				c.Write(zap.Object("subscriber", s), zap.Object("update", update))
			}
		}
	}
}

// registerSubscriber initializes the connection.
func (h *Hub) registerSubscriber(w http.ResponseWriter, r *http.Request) *Subscriber {
	s := NewSubscriber(retrieveLastEventID(r), h.logger)
	s.Debug = h.debug
	s.RemoteAddr = r.RemoteAddr
	var privateTopics []string

	if h.subscriberJWT != nil {
		claims, err := authorize(r, h.subscriberJWT, nil, h.cookieName)
		if claims != nil {
			s.Claims = claims
			privateTopics = claims.Mercure.Subscribe
		}
		if err != nil || (claims == nil && !h.anonymous) {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			if c := h.logger.Check(zap.InfoLevel, "Subscriber unauthorized"); c != nil {
				c.Write(zap.Object("subscriber", s), zap.Error(err))
			}

			return nil
		}
	}

	topics := r.URL.Query()["topic"]
	if len(topics) == 0 {
		http.Error(w, "Missing \"topic\" parameter.", http.StatusBadRequest)

		return nil
	}
	s.SetTopics(topics, privateTopics)

	h.dispatchSubscriptionUpdate(s, true)
	if err := h.transport.AddSubscriber(s); err != nil {
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		h.dispatchSubscriptionUpdate(s, false)
		if c := h.logger.Check(zap.ErrorLevel, "Unable to add subscriber"); c != nil {
			c.Write(zap.Object("subscriber", s), zap.Error(err))
		}

		return nil
	}

	sendHeaders(w, s)

	if c := h.logger.Check(zap.InfoLevel, "New subscriber"); c != nil {
		c.Write(zap.Object("subscriber", s))
	}
	h.metrics.SubscriberConnected(s)

	return s
}

// sendHeaders sends correct HTTP headers to create a keep-alive connection.
func sendHeaders(w http.ResponseWriter, s *Subscriber) {
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
	fmt.Fprint(w, ":\n")
	w.(http.Flusher).Flush()
}

// retrieveLastEventID extracts the Last-Event-ID from the corresponding HTTP header with a fallback on the query parameter.
func retrieveLastEventID(r *http.Request) string {
	if id := r.Header.Get("Last-Event-ID"); id != "" {
		return id
	}

	return r.URL.Query().Get("Last-Event-ID")
}

// Write sends the given string to the client.
// It returns false if the dispatch timed out.
// The current write cannot be cancelled because of https://github.com/golang/go/issues/16100
func (h *Hub) write(w io.Writer, s zapcore.ObjectMarshaler, data string) bool {
	if h.dispatchTimeout == 0 {
		fmt.Fprint(w, data)
		w.(http.Flusher).Flush()

		return true
	}

	done := make(chan struct{})
	go func() {
		fmt.Fprint(w, data)
		w.(http.Flusher).Flush()
		close(done)
	}()

	timeout := time.NewTimer(h.dispatchTimeout)
	defer timeout.Stop()
	select {
	case <-done:
		return true
	case <-timeout.C:
		if c := h.logger.Check(zap.WarnLevel, "Dispatch timeout reached"); c != nil {
			c.Write(zap.Object("subscriber", s))
		}

		return false
	}
}

func (h *Hub) shutdown(s *Subscriber) {
	// Notify that the client is closing the connection
	s.Disconnect()
	h.transport.RemoveSubscriber(s)
	h.dispatchSubscriptionUpdate(s, false)
	if c := h.logger.Check(zap.InfoLevel, "Subscriber disconnected"); c != nil {
		c.Write(zap.Object("subscriber", s))
	}
	h.metrics.SubscriberDisconnected(s)
}

func (h *Hub) dispatchSubscriptionUpdate(s *Subscriber, active bool) {
	if !h.subscriptions {
		return
	}

	for _, subscription := range s.getSubscriptions("", jsonldContext, active) {
		json, err := json.MarshalIndent(subscription, "", "  ")
		if err != nil {
			panic(err)
		}

		u := &Update{
			Topics:  []string{subscription.ID},
			Private: true,
			Debug:   h.debug,
			Event:   Event{Data: string(json)},
		}
		h.transport.Dispatch(u)
	}
}

func assertFlusher(w http.ResponseWriter) {
	if _, ok := w.(http.Flusher); !ok {
		panic("http.ResponseWriter must be an instance of http.Flusher")
	}
}
