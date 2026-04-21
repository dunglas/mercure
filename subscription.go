package mercure

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel/trace"
)

const (
	jsonldContext            = "https://mercure.rocks/"
	subscriptionsPath        = "/subscriptions"
	subscriptionURL          = defaultHubURL + subscriptionsPath + "/{topic}/{subscriber}"
	subscriptionsForTopicURL = defaultHubURL + subscriptionsPath + "/{topic}"
	subscriptionsURL         = defaultHubURL + subscriptionsPath
)

var jsonldContentType = []string{"application/ld+json"} // nolint:gochecknoglobals

type subscription struct {
	Context     string `json:"@context,omitempty"`
	ID          string `json:"id"`
	Type        string `json:"type"`
	Subscriber  string `json:"subscriber"`
	Topic       string `json:"topic"`
	Active      bool   `json:"active"`
	LastEventID string `json:"lastEventID,omitempty"`
	Payload     any    `json:"payload,omitempty"`
}

type subscriptionCollection struct {
	Context       string         `json:"@context"`
	ID            string         `json:"id"`
	Type          string         `json:"type"`
	LastEventID   string         `json:"lastEventID"`
	Subscriptions []subscription `json:"subscriptions"`
}

func (h *Hub) SubscriptionsHandler(w http.ResponseWriter, r *http.Request) {
	span, currentURL, lastEventID, subscribers, ok := h.initSubscription(w, r)
	defer span.End()

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

	j, err := json.MarshalIndent(subscriptionCollection, "", "  ")
	if err != nil {
		// Can't happen
		panic(err)
	}

	if _, err := w.Write(j); err != nil {
		ctx := r.Context()

		if h.logger.Enabled(ctx, slog.LevelInfo) {
			h.logger.LogAttrs(ctx, slog.LevelInfo, "Failed to write subscriptions response", slog.Any("error", err))
		}
	}
}

func (h *Hub) SubscriptionHandler(w http.ResponseWriter, r *http.Request) {
	span, _, lastEventID, subscribers, ok := h.initSubscription(w, r)
	defer span.End()

	if !ok {
		return
	}

	ctx := r.Context()
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

			j, err := json.MarshalIndent(subscription, "", "  ")
			if err != nil {
				panic(err)
			}

			if _, err := w.Write(j); err != nil && h.logger.Enabled(ctx, slog.LevelInfo) { //nolint:gosec
				h.logger.LogAttrs(ctx, slog.LevelInfo, "Failed to write subscription response", slog.Any("subscriber", subscriber), slog.Any("error", err))
			}

			return
		}
	}

	http.NotFound(w, r)
}

func (h *Hub) initSubscription(w http.ResponseWriter, r *http.Request) (span trace.Span, currentURL, lastEventID string, subscribers []*Subscriber, ok bool) {
	ctx, span := startSpan(r.Context(), "mercure.subscriptions", trace.WithSpanKind(trace.SpanKindInternal))
	currentURL = r.URL.RequestURI()

	if h.subscriberJWTKeyFunc != nil {
		claims, err := h.authorize(r, false)
		if err != nil || claims == nil || claims.Mercure.Subscribe == nil || !canReceive(h.topicSelectorStore, []string{currentURL}, claims.Mercure.Subscribe) {
			h.httpAuthorizationError(w, r, err)

			if err != nil {
				recordSpanError(span, err)
			}

			return span, "", "", nil, false
		}
	}

	transport, ok := h.transport.(TransportSubscribers)
	if !ok {
		panic("The transport isn't an instance of hub.TransportSubscribers")
	}

	var err error

	lastEventID, subscribers, err = transport.GetSubscribers(ctx)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)

		if h.logger.Enabled(ctx, slog.LevelError) {
			h.logger.LogAttrs(ctx, slog.LevelError, "Error retrieving subscribers", slog.Any("error", err))
		}

		recordSpanError(span, err)

		return span, currentURL, lastEventID, subscribers, false
	}

	if r.Header.Get("If-None-Match") == lastEventID {
		w.WriteHeader(http.StatusNotModified)

		return span, "", "", nil, false
	}

	header := w.Header()
	header["Content-Type"] = jsonldContentType
	header["ETag"] = []string{lastEventID}

	return span, currentURL, lastEventID, subscribers, true
}
