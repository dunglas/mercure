package hub

import (
	"fmt"
	"net/http"
	"strings"
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

	subscriber, updateChan, ok := h.initSubscription(w, r)
	if !ok {
		return
	}
	defer h.cleanup(subscriber)

	for {
		if h.options.HeartbeatInterval == time.Duration(0) {
			// No heartbeat defined, just block
			serializedUpdate, open := <-updateChan
			if !open {
				return
			}
			h.publish(serializedUpdate, subscriber, w, r)

			continue
		}

		select {
		case serializedUpdate, open := <-updateChan:
			if !open {
				return
			}
			h.publish(serializedUpdate, subscriber, w, r)

		case <-time.After(h.options.HeartbeatInterval):
			// Send a SSE comment as a heartbeat, to prevent issues with some proxies and old browsers
			fmt.Fprint(w, ":\n")
			f.Flush()
		}
	}
}

// initSubscription initializes the connection
func (h *Hub) initSubscription(w http.ResponseWriter, r *http.Request) (*Subscriber, chan *serializedUpdate, bool) {
	fields := log.Fields{"remote_addr": r.RemoteAddr}

	claims, err := authorize(r, h.options.SubscriberJWTKey, nil)
	if h.options.Debug && claims != nil {
		fields["target"] = claims.Mercure.Subscribe
	}

	if err != nil || (claims == nil && !h.options.AllowAnonymous) {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		log.WithFields(fields).Info(err)
		return nil, nil, false
	}

	topics := r.URL.Query()["topic"]
	if len(topics) == 0 {
		http.Error(w, "Missing \"topic\" parameter.", http.StatusBadRequest)
		return nil, nil, false
	}
	fields["subscriber_topics"] = topics

	var rawTopics = make([]string, 0, len(topics))
	var templateTopics = make([]*uritemplate.Template, 0, len(topics))
	for _, topic := range topics {
		if tpl := h.getURITemplate(topic); tpl == nil {
			rawTopics = append(rawTopics, topic)
		} else {
			templateTopics = append(templateTopics, tpl)
		}
	}

	sendHeaders(w)
	log.WithFields(fields).Info("New subscriber")

	authorizedAlltargets, authorizedTargets := authorizedTargets(claims, false)
	subscriber := NewSubscriber(authorizedAlltargets, authorizedTargets, topics, rawTopics, templateTopics, retrieveLastEventID(r))

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
		log.WithFields(fields).Info("Subscriber disconnected")
	}()

	return subscriber, updateChan, true
}

// getURITemplate retrieves or creates the uritemplate.Template associated with this topic, or nil if it's not a template
func (h *Hub) getURITemplate(topic string) *uritemplate.Template {
	var tpl *uritemplate.Template
	h.uriTemplates.Lock()
	if tplCache, ok := h.uriTemplates.m[topic]; ok {
		tpl = tplCache.template
		tplCache.counter = tplCache.counter + 1
	} else {
		if strings.Contains(topic, "{") { // If it's definitely not an URI template, skip to save some resources
			tpl, _ = uritemplate.New(topic) // Returns nil in case of error, will be considered as a raw string
		}

		h.uriTemplates.m[topic] = &templateCache{1, tpl}
	}
	h.uriTemplates.Unlock()

	return tpl
}

// sendHeaders sends correct HTTP headers to create a keep-alive connection
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

	// Write a comment in the body
	// Go currently doesn't provide a better way to flush the headers
	fmt.Fprint(w, ":\n")
	w.(http.Flusher).Flush()
}

// retrieveLastEventID extracts the Last-Event-ID from the corresponding HTTP header
// with a fallback on the query parameter
func retrieveLastEventID(r *http.Request) string {
	if id := r.Header.Get("Last-Event-ID"); id != "" {
		return id
	}

	return r.URL.Query().Get("Last-Event-ID")
}

// sendMissedEvents sends the events received since the one provided in Last-Event-ID
func (h *Hub) sendMissedEvents(w http.ResponseWriter, r *http.Request, s *Subscriber) {
	f := w.(http.Flusher)
	if err := h.history.FindFor(s, func(u *Update) bool {
		fmt.Fprint(w, u.String())
		f.Flush()
		log.WithFields(h.createLogFields(r, u, s)).Info("Event sent")

		return true
	}); err != nil {
		panic(err)
	}
}

// publish sends the update to the client, if authorized
func (h *Hub) publish(serializedUpdate *serializedUpdate, subscriber *Subscriber, w http.ResponseWriter, r *http.Request) {
	fields := h.createLogFields(r, serializedUpdate.Update, subscriber)

	if !subscriber.IsAuthorized(serializedUpdate.Update) {
		log.WithFields(fields).Debug("Subscriber not authorized to receive this update (no targets matching)")
		return
	}

	if !subscriber.IsSubscribed(serializedUpdate.Update) {
		log.WithFields(fields).Debug("Subscriber has not subscribed to this update (no topics matching)")
		return
	}

	fmt.Fprint(w, serializedUpdate.event)
	w.(http.Flusher).Flush()
	log.WithFields(fields).Info("Event sent")
}

// cleanup removes unused uritemplate.Template instances from memory
func (h *Hub) cleanup(s *Subscriber) {
	keys := make([]string, 0, len(s.RawTopics)+len(s.TemplateTopics))
	copy(s.RawTopics, keys)
	for _, uriTemplate := range s.TemplateTopics {
		keys = append(keys, uriTemplate.Raw())
	}

	h.uriTemplates.Lock()
	for _, key := range keys {
		counter := h.uriTemplates.m[key].counter
		if counter == 0 {
			delete(h.uriTemplates.m, key)
		} else {
			h.uriTemplates.m[key].counter = counter - 1
		}
	}
	h.uriTemplates.Unlock()
}
