package hub

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
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

const subscriptionURL = defaultHubURL + "/subscriptions/{topic}/{subscriber}"
const subscriptionsForTopicURL = defaultHubURL + "/subscriptions/{topic}"
const subscriptionsURL = defaultHubURL + "/subscriptions"

func (h *Hub) SubscriptionsHandler(w http.ResponseWriter, r *http.Request) {
	currentURL := r.URL.String()
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
	currentURL := r.URL.String()
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
	claims, err := authorize(r, h.getJWTKey(roleSubscriber), h.getJWTAlgorithm(roleSubscriber), nil)
	if err != nil || claims == nil || claims.Mercure.Subscribe == nil || !canReceive(h.topicSelectorStore, []string{currentURL}, claims.Mercure.Subscribe, false) {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		log.WithFields(log.Fields{"remote_addr": r.RemoteAddr}).Info(err)
		return "", nil, false
	}

	lastEventID, subscribers = h.transport.GetSubscribers()

	if r.Header.Get("If-None-Match") == lastEventID {
		w.WriteHeader(http.StatusNotModified)
		return "", nil, false
	}

	w.Header().Add("Content-Type", "application/ld+json")
	w.Header().Add("ETag", lastEventID)

	return lastEventID, subscribers, true
}
