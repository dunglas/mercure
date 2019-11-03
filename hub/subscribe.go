package hub

import (
	"context"
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

	subscriber, pipe, ok := h.initSubscription(w, r)
	if !ok {
		return
	}
	defer h.cleanup(subscriber)

	if h.options.HeartbeatInterval == time.Duration(0) {
		for {
			// No heartbeat defined, just block
			update, err := pipe.Read(context.Background())
			if err != nil {
				return
			}

			h.publish(newSerializedUpdate(update), subscriber, w, r)
		}
	}

	for {
		ctx, cancel := context.WithTimeout(context.Background(), h.options.HeartbeatInterval)
		update, err := pipe.Read(ctx)
		cancel()

		if err == context.DeadlineExceeded {
			// Send a SSE comment as a heartbeat, to prevent issues with some proxies and old browsers
			fmt.Fprint(w, ":\n")
			f.Flush()

			continue
		}

		if err != nil {
			return
		}

		h.publish(newSerializedUpdate(update), subscriber, w, r)
	}
}

// initSubscription initializes the connection
func (h *Hub) initSubscription(w http.ResponseWriter, r *http.Request) (*Subscriber, *Pipe, bool) {
	fields := log.Fields{"remote_addr": r.RemoteAddr}

	claims, err := authorize(r, h.options.SubscriberJWTKey, h.options.SubscriberJWTAlgorithm, nil)
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

	authorizedAlltargets, authorizedTargets := authorizedTargets(claims, false)
	subscriber := NewSubscriber(authorizedAlltargets, authorizedTargets, topics, rawTopics, templateTopics, retrieveLastEventID(r))

	pipe, err := h.transport.CreatePipe(subscriber.LastEventID)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		log.WithFields(fields).Error(err)
		return nil, nil, false
	}

	sendHeaders(w)
	log.WithFields(fields).Info("New subscriber")

	// Listen to the closing of the http connection via the Request's Context
	go func() {
		<-r.Context().Done()
		pipe.Close()
		log.WithFields(fields).Info("Subscriber disconnected")
	}()

	return subscriber, pipe, true
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
