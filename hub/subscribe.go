package hub

import (
	"context"
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

// SubscribeHandler create a keep alive connection and send the events to the subscribers
func (h *Hub) SubscribeHandler(w http.ResponseWriter, r *http.Request) {
	f, ok := w.(http.Flusher)
	if !ok {
		panic("http.ResponseWriter must be an instance of http.Flusher")
	}

	subscriber, pipe, ok := h.initSubscription(w, r)
	if !ok {
		return
	}
	defer h.cleanup(subscriber)

	hearthbeatInterval := h.config.GetDuration("heartbeat_interval")
	if hearthbeatInterval == time.Duration(0) {
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
		ctx, cancel := context.WithTimeout(context.Background(), hearthbeatInterval)
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

	claims, err := authorize(r, h.getJWTKey(subscriberRole), h.getJWTAlgorithm(subscriberRole), nil)
	if h.config.GetBool("debug") && claims != nil {
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
	subscriber := NewSubscriber(authorizedAlltargets, authorizedTargets, topics, rawTopics, templateTopics, retrieveLastEventID(r))

	encodedTopics := escapeTopics(topics)

	// Connection events must be sent before creating the pipe to prevent a deadlock
	connectionID := uuid.Must(uuid.NewV4()).String()
	var address string
	if h.config.GetBool("subscriptions_include_ip") {
		address, _, _ = net.SplitHostPort(r.RemoteAddr)
	}
	h.dispatchSubscriptionUpdate(topics, encodedTopics, connectionID, claims, true, address)
	pipe, err := h.transport.CreatePipe(subscriber.LastEventID)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		h.dispatchSubscriptionUpdate(topics, encodedTopics, connectionID, claims, false, address)
		log.WithFields(fields).Error(err)
		return nil, nil, false
	}
	sendHeaders(w)

	log.WithFields(fields).Info("New subscriber")

	// Listen to the closing of the http connection via the Request's Context
	go func() {
		<-r.Context().Done()
		pipe.Close()

		h.dispatchSubscriptionUpdate(topics, encodedTopics, connectionID, claims, false, address)
		log.WithFields(fields).Info("Subscriber disconnected")
	}()

	return subscriber, pipe, true
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

// getURITemplate retrieves or creates the uritemplate.Template associated with this topic, or nil if it's not a template
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

// sendHeaders sends correct HTTP headers to create a keep-alive connection
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

// retrieveLastEventID extracts the Last-Event-ID from the corresponding HTTP header
// with a fallback on the query parameter
func retrieveLastEventID(r *http.Request) string {
	if id := r.Header.Get("Last-Event-ID"); id != "" {
		return id
	}

	return r.URL.Query().Get("Last-Event-ID")
}

// publish sends the update to the client, if authorized
func (h *Hub) publish(serializedUpdate *serializedUpdate, subscriber *Subscriber, w io.Writer, r *http.Request) {
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

		h.transport.Write(u)
	}
}

func escapeTopics(topics []string) []string {
	encodedTopics := make([]string, 0, len(topics))
	for _, topic := range topics {
		encodedTopics = append(encodedTopics, url.QueryEscape(topic))
	}

	return encodedTopics
}
