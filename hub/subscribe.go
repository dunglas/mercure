package hub

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	log "github.com/sirupsen/logrus"
)

// SubscribeHandler creates a keep alive connection and sends the events to the subscribers.
func (h *Hub) SubscribeHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := w.(http.Flusher); !ok {
		panic("http.ResponseWriter must be an instance of http.Flusher")
	}

	debug := h.config.GetBool("debug")
	s := h.registerSubscriber(w, r, debug)
	if s == nil {
		return
	}
	defer h.shutdown(s)

	heartbeatInterval := h.config.GetDuration("heartbeat_interval")

	var heartbeatTimer *time.Timer
	var heartbeatTimerC <-chan time.Time
	if heartbeatInterval != time.Duration(0) {
		heartbeatTimer = time.NewTimer(heartbeatInterval)
		heartbeatTimerC = heartbeatTimer.C
	}

	dispatchTimeout := h.config.GetDuration("dispatch_timeout")
	writeTimeout := h.config.GetDuration("write_timeout")
	var writeTimer *time.Timer
	var writeTimerC <-chan time.Time
	if writeTimeout != 0 {
		writeTimer = time.NewTimer(writeTimeout - dispatchTimeout)
		writeTimerC = writeTimer.C
	}

	for {
		select {
		case <-r.Context().Done():
			// Client closes the connection
			return
		case <-writeTimerC:
			// Close properly the connection before the write timeout
			return
		case <-heartbeatTimerC:
			// Send a SSE comment as a heartbeat, to prevent issues with some proxies and old browsers
			if !h.write(w, s, ":\n", dispatchTimeout) {
				return
			}
			heartbeatTimer.Reset(heartbeatInterval)
		case update, ok := <-s.Receive():
			if !ok || !h.write(w, s, newSerializedUpdate(update).event, dispatchTimeout) {
				return
			}
			if heartbeatTimer != nil {
				if !heartbeatTimer.Stop() {
					<-heartbeatTimer.C
				}
				heartbeatTimer.Reset(heartbeatInterval)
			}
			log.WithFields(createFields(update, s)).Info("Event sent")
		}
	}
}

// registerSubscriber initializes the connection.
func (h *Hub) registerSubscriber(w http.ResponseWriter, r *http.Request, debug bool) *Subscriber {
	s := NewSubscriber(retrieveLastEventID(r), h.topicSelectorStore)
	s.Debug = debug
	s.LogFields["remote_addr"] = r.RemoteAddr

	claims, err := authorize(r, h.getJWTKey(roleSubscriber), h.getJWTAlgorithm(roleSubscriber), nil)
	if claims != nil {
		s.Claims = claims
		s.LogFields["subscriber_topic_selectors"] = claims.Mercure.Subscribe
	}
	if err != nil || (claims == nil && !h.config.GetBool("allow_anonymous")) {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		log.WithFields(s.LogFields).Info(err)
		return nil
	}

	s.Topics = r.URL.Query()["topic"]
	if len(s.Topics) == 0 {
		http.Error(w, "Missing \"topic\" parameter.", http.StatusBadRequest)
		return nil
	}
	s.LogFields["subscriber_topics"] = s.Topics
	s.EscapedTopics = escapeTopics(s.Topics)
	go s.start()

	h.dispatchSubscriptionUpdate(s, true)
	if err := h.transport.AddSubscriber(s); err != nil {
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		h.dispatchSubscriptionUpdate(s, false)
		log.WithFields(s.LogFields).Error(err)
		return nil
	}

	sendHeaders(w, s)

	log.WithFields(s.LogFields).Info("New subscriber")
	h.metrics.NewSubscriber(s)

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
func (h *Hub) write(w io.Writer, s *Subscriber, data string, d time.Duration) bool {
	if d == time.Duration(0) {
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

	select {
	case <-done:
		return true
	case <-time.After(d):
		log.WithFields(s.LogFields).Warn("Dispatch timeout reached")
		return false
	}
}

func (h *Hub) shutdown(s *Subscriber) {
	// Notify that the client is closing the connection
	s.Disconnect()
	h.dispatchSubscriptionUpdate(s, false)
	log.WithFields(s.LogFields).Info("Subscriber disconnected")
	h.metrics.SubscriberDisconnect(s)
}

func (h *Hub) dispatchSubscriptionUpdate(s *Subscriber, active bool) {
	if !h.config.GetBool("subscriptions") {
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
			Event:   Event{Data: string(json)},
		}
		h.transport.Dispatch(u)
	}
}

func escapeTopics(topics []string) []string {
	escapedTopics := make([]string, 0, len(topics))
	for _, topic := range topics {
		escapedTopics = append(escapedTopics, url.QueryEscape(topic))
	}

	return escapedTopics
}
