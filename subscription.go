package mercure

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
)

const (
	jsonldContext            = "https://mercure.rocks/"
	subscriptionsPath        = "/subscriptions"
	subscriptionURL          = defaultHubURL + subscriptionsPath + "/{topic}/{subscriber}"
	subscriptionsForTopicURL = defaultHubURL + subscriptionsPath + "/{topic}"
	subscriptionsURL         = defaultHubURL + subscriptionsPath

	// New URL patterns with matchType.
	subscriptionMatchURL     = defaultHubURL + subscriptionsPath + "/{matchType}/{match}/{subscriber}"
	subscriptionsForMatchURL = defaultHubURL + subscriptionsPath + "/{matchType}/{match}"
)

var jsonldContentType = []string{"application/ld+json"} // nolint:gochecknoglobals

type subscription struct {
	Context     string `json:"@context,omitempty"`
	ID          string `json:"id"`
	Type        string `json:"type"`
	Subscriber  string `json:"subscriber"`
	Topic       string `json:"topic,omitempty"`
	Match       string `json:"match,omitempty"`
	MatchType   string `json:"matchType,omitempty"`
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

// subscriptionFilter describes the filter to apply on a subscription listing,
// based on the URL path variables of the subscription API request.
//
// Either topic is set (deprecated URL /subscriptions/{topic}[/{subscriber}])
// or matchType+match are set (new URL /subscriptions/{matchType}/{match}[/{subscriber}]).
type subscriptionFilter struct {
	topic     string
	matchType string
	match     string
}

// filterFromVars builds a subscriptionFilter from mux path variables. Returns
// an error if any of the URL-encoded segments contains invalid escape
// sequences — the caller should answer 400 rather than silently serving an
// unfiltered listing.
func filterFromVars(vars map[string]string) (subscriptionFilter, error) {
	var f subscriptionFilter

	for _, seg := range []struct {
		name string
		dst  *string
	}{{paramTopic, &f.topic}, {paramMatch, &f.match}, {paramMatchType, &f.matchType}} {
		v, err := url.PathUnescape(vars[seg.name])
		if err != nil {
			return subscriptionFilter{}, errors.New("invalid " + seg.name + " segment: " + err.Error()) //nolint:err113
		}

		*seg.dst = v
	}

	return f, nil
}

func (h *Hub) SubscriptionsHandler(w http.ResponseWriter, r *http.Request) {
	currentURL := r.URL.RequestURI()

	filter, err := filterFromVars(mux.Vars(r))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)

		return
	}

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

	for _, subscriber := range subscribers {
		subscriptionCollection.Subscriptions = append(subscriptionCollection.Subscriptions, subscriber.getSubscriptions(filter, "", true)...)
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
	currentURL := r.URL.RequestURI()

	vars := mux.Vars(r)

	s, err := url.PathUnescape(vars["subscriber"])
	if err != nil {
		http.Error(w, "invalid subscriber segment: "+err.Error(), http.StatusBadRequest)

		return
	}

	filter, err := filterFromVars(vars)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)

		return
	}

	lastEventID, subscribers, ok := h.initSubscription(currentURL, w, r)
	if !ok {
		return
	}

	ctx := r.Context()

	for _, subscriber := range subscribers {
		if subscriber.ID != s {
			continue
		}

		for _, subscription := range subscriber.getSubscriptions(filter, jsonldContext, true) {
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

func (h *Hub) initSubscription(currentURL string, w http.ResponseWriter, r *http.Request) (lastEventID string, subscribers []*Subscriber, ok bool) {
	if h.subscriberJWTKeyFunc != nil {
		claims, err := h.authorize(r, false)
		if err != nil || claims == nil || claims.Mercure.Subscribe == nil {
			h.httpAuthorizationError(w, r, err)

			return "", nil, false
		}

		deprecated := h.isBackwardCompatiblyEnabledWith(8)
		if resolveErr := resolveMatcherClaims(h.topicSelectorStore, claims.Mercure.Subscribe, deprecated); resolveErr != nil {
			writeMatcherClaimError(r.Context(), h.logger, w, resolveErr)

			return "", nil, false
		}

		if !canReceive(h.topicSelectorStore, []string{currentURL}, claims.Mercure.Subscribe) {
			h.httpAuthorizationError(w, r, errors.New("subscription URL not covered by token topic matchers")) //nolint:err113

			return "", nil, false
		}
	}

	transport, isSubTransport := h.transport.(TransportSubscribers)
	if !isSubTransport {
		panic("The transport isn't an instance of hub.TransportSubscribers")
	}

	lastEventID, subscribers, err := transport.GetSubscribers(r.Context())
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)

		ctx := r.Context()
		if h.logger.Enabled(ctx, slog.LevelError) {
			h.logger.LogAttrs(ctx, slog.LevelError, "Error retrieving subscribers", slog.Any("error", err))
		}

		return "", nil, false
	}

	if r.Header.Get("If-None-Match") == lastEventID {
		w.WriteHeader(http.StatusNotModified)

		return "", nil, false
	}

	header := w.Header()
	header["Content-Type"] = jsonldContentType
	header["ETag"] = []string{lastEventID}

	return lastEventID, subscribers, true
}
