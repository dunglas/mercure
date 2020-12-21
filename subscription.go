package mercure

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

const jsonldContext = "https://mercure.rocks/"

type subscription struct {
	Context     string      `json:"@context,omitempty"`
	ID          string      `json:"id"`
	Type        string      `json:"type"`
	Subscriber  string      `json:"subscriber"`
	Topic       string      `json:"topic"`
	Active      bool        `json:"active"`
	LastEventID string      `json:"lastEventID,omitempty"`
	Payload     interface{} `json:"payload,omitempty"`
}

type subscriptionCollection struct {
	Context       string         `json:"@context"`
	ID            string         `json:"id"`
	Type          string         `json:"type"`
	LastEventID   string         `json:"lastEventID"`
	Subscriptions []subscription `json:"subscriptions"`
}

const (
	subscriptionURL          = defaultHubURL + "/subscriptions/{topic}/{subscriber}"
	subscriptionsForTopicURL = defaultHubURL + "/subscriptions/{topic}"
	subscriptionsURL         = defaultHubURL + "/subscriptions"
)

func (h *Hub) SubscriptionsHandler(w http.ResponseWriter, r *http.Request) {
	currentURL := r.URL.RequestURI()
	lastEventID, subscribers, ok := h.initSubscription(currentURL, w, r)
	if !ok {
		return
	}
	w.WriteHeader(http.StatusOK)

	subscriptionCollection := subscriptionCollection{
		Context:       jsonldContext,
		ID:            currentURL,
		Type:          "Subscriptions",
		LastEventID:   lastEventID,
		Subscriptions: make([]subscription, 0),
	}

	vars := mux.Vars(r)
	t, _ := url.QueryUnescape(vars["topic"])
	for _, subscriber := range subscribers {
		subscriptionCollection.Subscriptions = append(subscriptionCollection.Subscriptions, subscriber.getSubscriptions(t, "", true)...)
	}

	json, err := json.MarshalIndent(subscriptionCollection, "", "  ")
	if err != nil {
		panic(err)
	}

	w.Write(json)
}

func (h *Hub) SubscriptionHandler(w http.ResponseWriter, r *http.Request) {
	currentURL := r.URL.RequestURI()
	lastEventID, subscribers, ok := h.initSubscription(currentURL, w, r)
	if !ok {
		return
	}

	vars := mux.Vars(r)
	s, _ := url.QueryUnescape(vars["subscriber"])
	t, _ := url.QueryUnescape(vars["topic"])

	for _, subscriber := range subscribers {
		if subscriber.ID != s {
			continue
		}

		for _, subscription := range subscriber.getSubscriptions(t, jsonldContext, true) {
			if subscription.Topic != t {
				continue
			}

			subscription.LastEventID = lastEventID
			json, err := json.MarshalIndent(subscription, "", "  ")
			if err != nil {
				panic(err)
			}

			w.Write(json)

			return
		}
	}

	w.WriteHeader(http.StatusNotFound)
}

func (h *Hub) initSubscription(currentURL string, w http.ResponseWriter, r *http.Request) (lastEventID string, subscribers []*Subscriber, ok bool) {
	if h.subscriberJWT != nil {
		claims, err := authorize(r, h.subscriberJWT, nil)
		if err != nil || claims == nil || claims.Mercure.Subscribe == nil || !canReceive(h.topicSelectorStore, []string{currentURL}, claims.Mercure.Subscribe) {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			h.logger.Info("Topic selectors not matched, not provided or authorization error", zap.String("remote_addr", r.RemoteAddr), zap.Error(err))

			return "", nil, false
		}
	}

	transport, ok := h.transport.(TransportSubscribers)
	if !ok {
		panic("The transport isn't an instance of hub.TransportSubscribers")
	}

	lastEventID, subscribers = transport.GetSubscribers()
	if r.Header.Get("If-None-Match") == lastEventID {
		w.WriteHeader(http.StatusNotModified)

		return "", nil, false
	}

	w.Header().Add("Content-Type", "application/ld+json")
	w.Header().Add("ETag", lastEventID)

	return lastEventID, subscribers, true
}
