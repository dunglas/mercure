package mercure

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel/trace"
)

const (
	subscriptionsPath = "/subscriptions"
	subscriptionsURL  = defaultHubURL + subscriptionsPath

	subscriptionMatchURL     = defaultHubURL + subscriptionsPath + "/{match_type}/{match}/{subscriber}"
	subscriptionsForMatchURL = defaultHubURL + subscriptionsPath + "/{match_type}/{match}"

	// reservedEventType is the SSE "event" field value the hub sets on every
	// update it generates itself (currently subscription events). Publishers
	// are forbidden from using it (see Update.Validate) so that a client
	// listening for it over a shared connection cannot receive forged events.
	reservedEventType = "mercure"
)

var subscriptionContentType = []string{"application/mercure+json"} // nolint:gochecknoglobals

// etagValue encodes lastEventID as the content of an RFC 9110 §8.8.3
// entity-tag. Bytes outside etagc (SP, DQUOTE, C0/C1 controls, DEL) are
// percent-encoded so the header stays syntactically valid even for legacy IDs
// persisted before publish-time validation rejected them. "%" is its own escape
// marker, so it is encoded too, keeping the mapping injective.
func etagValue(lastEventID string) string {
	var b strings.Builder

	for i := range len(lastEventID) {
		if c := lastEventID[i]; c < 0x21 || c == '"' || c == '%' || c == 0x7f {
			const hex = "0123456789ABCDEF"

			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&0x0f])
		} else {
			b.WriteByte(lastEventID[i])
		}
	}

	return b.String()
}

type subscription struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Subscriber  string `json:"subscriber"`
	Topic       string `json:"topic,omitempty"`
	Match       string `json:"match,omitempty"`
	MatchType   string `json:"match_type,omitempty"`
	Active      bool   `json:"active"`
	LastEventID string `json:"last_event_id,omitempty"`
	Payload     any    `json:"payload,omitempty"`
}

type subscriptionCollection struct {
	ID            string         `json:"id"`
	Type          string         `json:"type"`
	LastEventID   string         `json:"last_event_id"`
	Subscriptions []subscription `json:"subscriptions"`
}

// subscriptionFilter describes the filter to apply on a subscription listing,
// based on the URL path variables of the subscription API request.
//
// Either topic is set (deprecated URL /subscriptions/{topic}[/{subscriber}])
// or match_type+match are set (URL /subscriptions/{match_type}/{match}[/{subscriber}]).
type subscriptionFilter struct {
	topic      string
	match_type string
	match      string
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
	}{{paramTopic, &f.topic}, {paramMatch, &f.match}, {paramMatchType, &f.match_type}} {
		v, err := url.PathUnescape(vars[seg.name])
		if err != nil {
			return subscriptionFilter{}, errors.New("invalid " + seg.name + " segment: " + err.Error()) //nolint:err113
		}

		*seg.dst = v
	}

	// Reject unknown matcher types with a 400 instead of silently serving an
	// empty listing. match_type is empty on the deprecated /{topic} routes.
	if f.match_type != "" && !knownMatcherType(MatcherType(f.match_type)) {
		return subscriptionFilter{}, ErrUnsupportedMatcherType
	}

	return f, nil
}

func (h *Hub) SubscriptionsHandler(w http.ResponseWriter, r *http.Request) {
	// Validate the URL shape before authorizing or fetching subscribers, so a
	// malformed request answers 400 without any response headers being set.
	filter, err := filterFromVars(mux.Vars(r))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)

		return
	}

	span, currentURL, last_event_id, subscribers, ok := h.initSubscription(w, r)
	defer span.End()

	if !ok {
		return
	}

	w.WriteHeader(http.StatusOK)

	subscriptionCollection := subscriptionCollection{
		ID:            currentURL,
		Type:          "subscriptions",
		LastEventID:   last_event_id,
		Subscriptions: make([]subscription, 0),
	}

	for _, subscriber := range subscribers {
		subscriptionCollection.Subscriptions = append(subscriptionCollection.Subscriptions, subscriber.getSubscriptions(filter, true)...)
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
	// Validate the URL shape before authorizing or fetching subscribers, so a
	// malformed request answers 400 without any response headers being set.
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

	span, _, last_event_id, subscribers, ok := h.initSubscription(w, r)
	defer span.End()

	if !ok {
		return
	}

	ctx := r.Context()

	for _, subscriber := range subscribers {
		if subscriber.ID != s {
			continue
		}

		for _, subscription := range subscriber.getSubscriptions(filter, true) {
			subscription.LastEventID = last_event_id

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

// authorizeSubscriptionRequest checks the subscriber token against the
// subscription API URL, writing the HTTP error response on failure.
func (h *Hub) authorizeSubscriptionRequest(ctx context.Context, span trace.Span, currentURL string, w http.ResponseWriter, r *http.Request) bool {
	if h.subscriberJWTKeyFunc == nil {
		return true
	}

	claims, err := h.authorize(r, false)
	if err != nil || claims == nil {
		h.httpAuthorizationError(w, r, err)

		if err != nil {
			recordSpanError(span, err)
		}

		return false
	}

	if resolveErr := resolveMatcherClaims(h.topicSelectorStore, claims.Mercure.Subscribe, h.allowsAlternateTopics()); resolveErr != nil {
		writeMatcherClaimError(ctx, h.logger, w, resolveErr)
		recordSpanError(span, resolveErr)

		return false
	}

	// The token is valid; if it does not grant subscribe on this URL the
	// failure is insufficient_scope (403), not an authentication error (401).
	if claims.Mercure.Subscribe == nil || !canReceive(h.topicSelectorStore, []string{currentURL}, claims.Mercure.Subscribe) {
		h.httpInsufficientScopeError(w, r, errors.New("subscription URL not covered by token topic matchers")) //nolint:err113

		return false
	}

	return true
}

func (h *Hub) initSubscription(w http.ResponseWriter, r *http.Request) (span trace.Span, currentURL, last_event_id string, subscribers []*Subscriber, ok bool) {
	ctx, span := startSpan(r.Context(), "mercure.subscriptions", trace.WithSpanKind(trace.SpanKindInternal))
	// The topic to authorize (and the collection id) is the absolute path in
	// relative form; RequestURI() would append the query string, so a token
	// presented via the access_token query parameter would no longer match an
	// Exact matcher byte-for-byte.
	currentURL = r.URL.EscapedPath()

	if !h.authorizeSubscriptionRequest(ctx, span, currentURL, w, r) {
		return span, "", "", nil, false
	}

	transport, isSubTransport := h.transport.(TransportSubscribers)
	if !isSubTransport {
		panic("The transport isn't an instance of hub.TransportSubscribers")
	}

	var err error

	last_event_id, subscribers, err = transport.GetSubscribers(ctx)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)

		if h.logger.Enabled(ctx, slog.LevelError) {
			h.logger.LogAttrs(ctx, slog.LevelError, "Error retrieving subscribers", slog.Any("error", err))
		}

		recordSpanError(span, err)

		return span, currentURL, last_event_id, subscribers, false
	}

	// ETags are entity-tags (RFC 9110 §8.8.3): DQUOTE-wrapped etagc bytes.
	// etagValue percent-encodes anything outside etagc so the header is valid
	// even for legacy IDs that predate publish-time validation.
	etag := `"` + etagValue(last_event_id) + `"`
	// A 304 carries the ETag it would have sent on a 200 (RFC 9110 §15.4.5), so
	// set it before the conditional check.
	header := w.Header()
	header["ETag"] = []string{etag}

	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)

		return span, "", "", nil, false
	}

	header["Content-Type"] = subscriptionContentType

	return span, currentURL, last_event_id, subscribers, true
}
