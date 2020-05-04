package hub

import (
	"context"
	"encoding/json"
	"errors"
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

// SubscribeHandler create a keep alive connection and send the events to the subscribers.
func (h *Hub) SubscribeHandler(w http.ResponseWriter, r *http.Request) {
	_, ok := w.(http.Flusher)
	if !ok {
		panic("http.ResponseWriter must be an instance of http.Flusher")
	}

	subscriber, unsubscribed, ok := h.initSubscription(w, r)
	if !ok {
		return
	}
	defer h.cleanup(subscriber)
	defer unsubscribed()
	// Notify that the client is closing the connection
	defer close(subscriber.ClientDisconnect)

	hearthbeatInterval := h.config.GetDuration("heartbeat_interval")
	var cancelHearthbeatTimeout context.CancelFunc
	for {
		ctxHearthbeat := context.Background()
		if hearthbeatInterval != time.Duration(0) {
			ctxHearthbeat, cancelHearthbeatTimeout = context.WithTimeout(ctxHearthbeat, hearthbeatInterval)
			defer cancelHearthbeatTimeout()
		}

		select {
		case <-subscriber.ServerDisconnect:
			// Server closes the connection
			return
		case <-r.Context().Done():
			// Client closes the connection
			return
		case <-ctxHearthbeat.Done():
			// Send a SSE comment as a heartbeat, to prevent issues with some proxies and old browsers
			if errors.Is(ctxHearthbeat.Err(), context.DeadlineExceeded) && !h.write(w, r, subscriber, ":\n") {
				return
			}
		case update := <-subscriber.Out:
			if !h.write(w, r, subscriber, newSerializedUpdate(update).event) {
				return
			}
			if cancelHearthbeatTimeout != nil {
				cancelHearthbeatTimeout()
			}

			fields := createBaseLogFields(subscriber.debug, r.RemoteAddr, update, subscriber)
			log.WithFields(fields).Info("Event sent")
		}
	}
}

// initSubscription initializes the connection.
func (h *Hub) initSubscription(w http.ResponseWriter, r *http.Request) (*Subscriber, func(), bool) {
	fields := log.Fields{"remote_addr": r.RemoteAddr}

	claims, err := authorize(r, h.getJWTKey(subscriberRole), h.getJWTAlgorithm(subscriberRole), nil)
	debug := h.config.GetBool("debug")
	if debug && claims != nil {
		fields["target"] = claims.Mercure.Subscribe
	}
	if err != nil || (claims == nil && !h.config.GetBool("allow_anonymous")) {
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

	rawTopics, templateTopics := h.parseTopics(topics)

	authorizedAlltargets, authorizedTargets := authorizedTargets(claims, false)
	subscriber := NewSubscriber(authorizedAlltargets, authorizedTargets, topics, rawTopics, templateTopics, retrieveLastEventID(r), r.RemoteAddr, debug)
	encodedTopics := escapeTopics(topics)

	// TODO: move this to the subscriber struct
	connectionID := uuid.Must(uuid.NewV4()).String()
	var address string
	if h.config.GetBool("subscriptions_include_ip") {
		address, _, _ = net.SplitHostPort(r.RemoteAddr)
	}
	// TODO: dispatchSubscriptionUpdate(subscriber)
	h.dispatchSubscriptionUpdate(topics, encodedTopics, connectionID, claims, true, address)
	if h.transport.AddSubscriber(subscriber) != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		h.dispatchSubscriptionUpdate(topics, encodedTopics, connectionID, claims, false, address)
		log.WithFields(fields).Error(err)
		return nil, nil, false
	}
	sendHeaders(w)
	log.WithFields(fields).Info("New subscriber")

	h.metrics.NewSubscriber(subscriber)

	unsubscribed := func() {
		h.dispatchSubscriptionUpdate(topics, encodedTopics, connectionID, claims, false, address)
		log.WithFields(fields).Info("Subscriber disconnected")

		h.metrics.SubscriberDisconnect(subscriber)
	}

	return subscriber, unsubscribed, true
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
		fields := createBaseLogFields(s.debug, r.RemoteAddr, nil, s)
		log.WithFields(fields).Warn("Dispatch timeout reached")
		return false
	}
}

// cleanup removes unused uritemplate.Template instances from memory.
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

func (h *Hub) dispatchSubscriptionUpdate(topics, encodedTopics []string, connectionID string, claims *claims, active bool, address string) {
	if !h.config.GetBool("dispatch_subscriptions") {
		return
	}

	for k, topic := range topics {
		connection := &subscription{
			ID:      "https://mercure.rocks/subscriptions/" + encodedTopics[k] + "/" + connectionID,
			Type:    "https://mercure.rocks/Subscription",
			Topic:   topic,
			Active:  active,
			Address: address,
		}

		if claims == nil {
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
			Targets: map[string]struct{}{"https://mercure.rocks/targets/subscriptions": {}, "https://mercure.rocks/targets/subscriptions/" + encodedTopics[k]: {}},
			Event:   Event{Data: string(json), ID: uuid.Must(uuid.NewV4()).String()},
		}

		h.transport.Dispatch(u)
	}
}

func escapeTopics(topics []string) []string {
	encodedTopics := make([]string, 0, len(topics))
	for _, topic := range topics {
		encodedTopics = append(encodedTopics, url.QueryEscape(topic))
	}

	return encodedTopics
}
