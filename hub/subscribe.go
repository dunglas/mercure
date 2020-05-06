package hub

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/yosida95/uritemplate"
)

type subscription struct {
	ID     string `json:"@id"`
	Type   string `json:"@type"`
	Topic  string `json:"topic"`
	Active bool   `json:"active"`
	mercureClaim
	Address string `json:"address,omitempty"`
}

// SubscribeHandler creates a keep alive connection and sends the events to the subscribers.
func (h *Hub) SubscribeHandler(w http.ResponseWriter, r *http.Request) {
	_, ok := w.(http.Flusher)
	if !ok {
		panic("http.ResponseWriter must be an instance of http.Flusher")
	}

	debug := h.config.GetBool("debug")
	s := h.registerSubscriber(w, r, debug)
	if s == nil {
		return
	}
	defer h.shutdown(s)

	hearthbeatInterval := h.config.GetDuration("heartbeat_interval")

	var timer *time.Timer
	var timerC <-chan time.Time
	if hearthbeatInterval != time.Duration(0) {
		timer = time.NewTimer(hearthbeatInterval)
		timerC = timer.C
	}

	for {
		select {
		case <-s.disconnected:
			// Server closes the connection
			return
		case <-r.Context().Done():
			// Client closes the connection
			return
		case <-timerC:
			// Send a SSE comment as a heartbeat, to prevent issues with some proxies and old browsers
			if !h.write(w, r, s, ":\n") {
				return
			}
			timer.Reset(hearthbeatInterval)
		case update := <-s.Out:
			if !h.write(w, r, s, newSerializedUpdate(update).event) {
				return
			}
			if timer != nil {
				if !timer.Stop() {
					<-timer.C
				}
				timer.Reset(hearthbeatInterval)
			}
			log.WithFields(createFields(update, s)).Info("Event sent")
		}
	}
}

// registerSubscriber initializes the connection.
func (h *Hub) registerSubscriber(w http.ResponseWriter, r *http.Request, debug bool) *Subscriber {
	s := newSubscriber()
	s.debug = debug
	s.logFields["remote_addr"] = r.RemoteAddr

	claims, err := authorize(r, h.getJWTKey(subscriberRole), h.getJWTAlgorithm(subscriberRole), nil)
	if claims != nil {
		s.claims = claims
		s.logFields["subscriber_targets"] = claims.Mercure.Subscribe
	}
	if err != nil || (claims == nil && !h.config.GetBool("allow_anonymous")) {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		log.WithFields(s.logFields).Info(err)
		return nil
	}

	s.topics = r.URL.Query()["topic"]
	if len(s.topics) == 0 {
		http.Error(w, "Missing \"topic\" parameter.", http.StatusBadRequest)
		return nil
	}
	s.logFields["subscriber_topics"] = s.topics

	s.rawTopics, s.templateTopics = h.parseTopics(s.topics)
	s.escapedTopics = escapeTopics(s.topics)
	s.allTargets, s.targets = authorizedTargets(claims, false)
	s.remoteAddr = r.RemoteAddr

	s.lastEventID = retrieveLastEventID(r)
	if s.lastEventID != "" {
		s.History.In = make(chan *Update)
		s.logFields["last_event_id"] = s.lastEventID
	}
	go s.start()

	if h.config.GetBool("subscriptions_include_ip") {
		s.remoteHost, _, _ = net.SplitHostPort(r.RemoteAddr)
	}
	h.dispatchSubscriptionUpdate(s, true)
	if h.transport.AddSubscriber(s) != nil {
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		h.dispatchSubscriptionUpdate(s, false)
		log.WithFields(s.logFields).Error(err)
		return nil
	}
	sendHeaders(w)
	log.WithFields(s.logFields).Info("New subscriber")

	h.metrics.NewSubscriber(s)

	return s
}

func (h *Hub) parseTopics(topics []string) (rawTopics []string, templateTopics []*uritemplate.Template) {
	rawTopics = make([]string, 0, len(topics))
	templateTopics = make([]*uritemplate.Template, 0, len(topics))
	for _, topic := range topics {
		if tpl := h.getURITemplate(topic); tpl == nil {
			rawTopics = append(rawTopics, topic)
		} else {
			templateTopics = append(templateTopics, tpl)
		}
	}

	return rawTopics, templateTopics
}

// getURITemplate retrieves or creates the uritemplate.Template associated with this topic, or nil if it's not a template.
func (h *Hub) getURITemplate(topic string) *uritemplate.Template {
	var tpl *uritemplate.Template
	h.uriTemplates.Lock()
	if tplCache, ok := h.uriTemplates.m[topic]; ok {
		tpl = tplCache.template
		tplCache.counter++
	} else {
		if strings.Contains(topic, "{") { // If it's definitely not an URI template, skip to save some resources
			tpl, _ = uritemplate.New(topic) // Returns nil in case of error, will be considered as a raw string
		}

		h.uriTemplates.m[topic] = &templateCache{1, tpl}
	}
	h.uriTemplates.Unlock()

	return tpl
}

// sendHeaders sends correct HTTP headers to create a keep-alive connection.
func sendHeaders(w http.ResponseWriter) {
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
func (h *Hub) write(w io.Writer, r *http.Request, s *Subscriber, data string) bool {
	d := h.config.GetDuration("dispatch_timeout")
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
		log.WithFields(s.logFields).Warn("Dispatch timeout reached")
		return false
	}
}

func (h *Hub) shutdown(s *Subscriber) {
	// Notify that the client is closing the connection
	s.Disconnect()
	h.dispatchSubscriptionUpdate(s, false)
	log.WithFields(s.logFields).Info("Subscriber disconnected")
	h.metrics.SubscriberDisconnect(s)

	// Remove unused uritemplate.Template instances from memory.
	keys := make([]string, 0, len(s.rawTopics)+len(s.templateTopics))
	copy(s.rawTopics, keys)
	for _, uriTemplate := range s.templateTopics {
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

func (h *Hub) dispatchSubscriptionUpdate(s *Subscriber, active bool) {
	if !h.config.GetBool("dispatch_subscriptions") {
		return
	}

	for k, topic := range s.topics {
		connection := &subscription{
			ID:      "https://mercure.rocks/subscriptions/" + s.escapedTopics[k] + "/" + s.ID,
			Type:    "https://mercure.rocks/Subscription",
			Topic:   topic,
			Active:  active,
			Address: s.remoteHost,
		}

		if s.claims == nil {
			connection.mercureClaim.Publish = []string{}
			connection.mercureClaim.Subscribe = []string{}
		} else {
			if connection.mercureClaim.Publish == nil {
				connection.mercureClaim.Publish = []string{}
			}
			if connection.mercureClaim.Subscribe == nil {
				connection.mercureClaim.Subscribe = []string{}
			}
		}

		json, err := json.MarshalIndent(connection, "", "  ")
		if err != nil {
			panic(err)
		}

		u := &Update{
			Topics:  []string{connection.ID},
			Targets: map[string]struct{}{"https://mercure.rocks/targets/subscriptions": {}, "https://mercure.rocks/targets/subscriptions/" + s.escapedTopics[k]: {}},
			Event:   Event{Data: string(json), ID: uuid.Must(uuid.NewV4()).String()},
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
