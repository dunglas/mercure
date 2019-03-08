package hub

import (
	"fmt"
	"net/http"
	"regexp"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/yosida95/uritemplate"
)

// SubscribeHandler create a keep alive connection and send the events to the subscribers
func (h *Hub) SubscribeHandler(w http.ResponseWriter, r *http.Request) {
	f, ok := w.(http.Flusher)
	if !ok {
		panic("The Response Writer must be an instance of Flusher.")
	}

	claims, err := authorize(r, h.options.SubscriberJWTKey, nil)
	if err != nil || (claims == nil && !h.options.AllowAnonymous) {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	topics := r.URL.Query()["topic"]
	if len(topics) == 0 {
		http.Error(w, "Missing \"topic\" parameter.", http.StatusBadRequest)
		return
	}

	var regexps = make([]*regexp.Regexp, len(topics))
	for index, topic := range topics {
		tpl, err := uritemplate.New(topic)
		if nil != err {
			http.Error(w, fmt.Sprintf("\"%s\" is not a valid URI template (RFC6570).", topic), http.StatusBadRequest)
			return
		}
		regexps[index] = tpl.Regexp()
	}

	log.WithFields(log.Fields{"remote_addr": r.RemoteAddr}).Info("New subscriber")
	sendHeaders(w)

	authorizedAlltargets, authorizedTargets := authorizedTargets(claims, false)
	subscriber := &Subscriber{authorizedAlltargets, authorizedTargets, regexps, retrieveLastEventID(r)}

	if subscriber.LastEventID != "" {
		h.sendMissedEvents(w, r, subscriber)
	}

	// Create a new channel, over which the hub can send can send updates to this subscriber.
	updateChan := make(chan *serializedUpdate)

	// Add this client to the map of those that should
	// receive updates
	h.newSubscribers <- updateChan

	// Listen to the closing of the http connection via the CloseNotifier
	notify := w.(http.CloseNotifier).CloseNotify()
	go func() {
		<-notify
		h.removedSubscribers <- updateChan
		log.WithFields(log.Fields{"remote_addr": r.RemoteAddr}).Info("Subscriber disconnected")
	}()

	publish := func(serializedUpdate *serializedUpdate) {
		// Check authorization
		if !subscriber.CanReceive(serializedUpdate.Update) {
			return
		}

		fmt.Fprint(w, serializedUpdate.event)
		log.WithFields(log.Fields{
			"event_id":    serializedUpdate.ID,
			"remote_addr": r.RemoteAddr,
		}).Info("Event sent")
		f.Flush()
	}

	for {
		if h.options.HeartbeatInterval == time.Duration(0) {
			// No heartbeat defined, just block
			serializedUpdate, open := <-updateChan
			if !open {
				return
			}
			publish(serializedUpdate)

			continue
		}

		select {
		case serializedUpdate, open := <-updateChan:
			if !open {
				return
			}
			publish(serializedUpdate)

		case <-time.After(h.options.HeartbeatInterval):
			// Send a SSE comment as a heartbeat, to prevent issues with some proxies and old browsers
			fmt.Fprint(w, ":\n")
			f.Flush()
		}
	}
}

// sendHeaders send correct HTTP headers to create a keep-alive connection
func sendHeaders(w http.ResponseWriter) {
	// Keep alive, useful only for HTTP 1 clients https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Keep-Alive
	w.Header().Set("Connection", "keep-alive")

	// Server-sent events https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events#Sending_events_from_the_server
	w.Header().Set("Content-Type", "text/event-stream")

	// Disable cache, even for old browsers and proxies
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expire", "0")

	// NGINX support https://www.nginx.com/resources/wiki/start/topics/examples/x-accel/#x-accel-buffering
	w.Header().Set("X-Accel-Buffering", "no")
	w.(http.Flusher).Flush()
}

func retrieveLastEventID(r *http.Request) string {
	if id := r.Header.Get("Last-Event-ID"); id != "" {
		return id
	}

	return r.URL.Query().Get("Last-Event-ID")
}

func (h *Hub) sendMissedEvents(w http.ResponseWriter, r *http.Request, s *Subscriber) {
	f := w.(http.Flusher)
	if err := h.history.FindFor(s, func(u *Update) bool {
		fmt.Fprint(w, u.String())
		f.Flush()
		log.WithFields(log.Fields{
			"event_id":      u.ID,
			"last_event_id": s.LastEventID,
			"remote_addr":   r.RemoteAddr,
		}).Info("Event sent")
		return true
	}); err != nil {
		panic(err)
	}
}
