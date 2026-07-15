package mercure

import (
	"log/slog"
	"net/url"
	"strings"
)

// Subscriber represents a client subscribed to a list of topics on a remote or on the current hub.
type Subscriber struct {
	ID                 string
	EscapedID          string
	Claims             *claims
	RequestLastEventID string

	// SubscribedMatchers are the topic matchers from the topic and
	// match_urlpattern query parameters (or from the v8 `topic` parameter,
	// which resolves to a deprecated matcher under compatibility mode).
	SubscribedMatchers []TopicMatcher
	// AllowedPrivateMatchers are the topic matchers from the JWT claims.
	AllowedPrivateMatchers []TopicMatcher
	// EscapedMatchers are precomputed "escapedType/escapedPattern" slugs for
	// subscription URLs.
	EscapedMatchers []string
	// SubscriptionPayloads holds the JSON-LD `payload` resolved per
	// subscribed matcher at registration time, indexed parallel to
	// SubscribedMatchers. Precomputing lets the subscription API render
	// payloads for subscribers a transport reconstructed from its
	// persistence layer without doing live matcher dispatch on the
	// deserialized object.
	SubscriptionPayloads []any

	logger             *slog.Logger
	topicSelectorStore *TopicSelectorStore
}

func NewSubscriber(logger *slog.Logger, topicSelectorStore *TopicSelectorStore) *Subscriber {
	return &Subscriber{
		logger:             logger,
		topicSelectorStore: topicSelectorStore,
	}
}

// MatchTopics checks if the current subscriber can access to at least one of the given topics.
func (s *Subscriber) MatchTopics(topics []string, private bool) bool {
	if !s.matchesAny(topics, s.SubscribedMatchers) {
		return false
	}

	return !private || s.matchesAny(topics, s.AllowedPrivateMatchers)
}

// Match checks if the current subscriber can receive the given update.
func (s *Subscriber) Match(u *Update) bool {
	return s.MatchTopics(u.topics(), u.Private)
}

func (s *Subscriber) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("id", s.ID),
		slog.String("last_event_id", s.RequestLastEventID),
	}

	if len(s.AllowedPrivateMatchers) != 0 {
		attrs = append(attrs, slog.Any("allowed_private_matchers", logMatcherPatterns(s.AllowedPrivateMatchers)))
	}

	if len(s.SubscribedMatchers) != 0 {
		attrs = append(attrs, slog.Any("subscribed_matchers", logMatcherPatterns(s.SubscribedMatchers)))
	}

	return slog.GroupValue(attrs...)
}

// SetMatchers sets the subscribed and allowed-private topic matchers and
// recomputes the derived subscription slugs and payloads, keeping the parallel
// SubscribedMatchers / EscapedMatchers / SubscriptionPayloads slices
// consistent. Transport implementations that reconstruct a Subscriber on
// another node use it to populate matchers programmatically, instead of
// relying on the exported fields (whose parallel-slice invariant is not
// otherwise enforced).
func (s *Subscriber) SetMatchers(subscribed, allowedPrivate []TopicMatcher) {
	s.setMatchers(subscribed, allowedPrivate)
}

func (s *Subscriber) matchesAny(topics []string, matchers []TopicMatcher) bool {
	for _, m := range matchers {
		if s.topicSelectorStore.matchMatcher(topics, m) {
			return true
		}
	}

	return false
}

func logMatcherPatterns(matchers []TopicMatcher) []string {
	out := make([]string, len(matchers))
	for i, m := range matchers {
		out[i] = string(m.Type) + ":" + m.Pattern
	}

	return out
}

// setMatchers sets the subscribed and allowed private topic matchers, and
// precomputes the per-matcher subscription payload so the subscription
// API can render it from a serialized Subscriber without re-running the
// matcher dispatch.
func (s *Subscriber) setMatchers(subscribed, allowedPrivate []TopicMatcher) {
	s.SubscribedMatchers = subscribed
	s.AllowedPrivateMatchers = allowedPrivate
	s.recomputeEscapedMatchers()
	s.resolveSubscriptionPayloads()
}

// recomputeEscapedMatchers builds the URL slug used in subscription IDs for
// each entry of SubscribedMatchers: "{escapedType}/{escapedPattern}" for
// modern matchers, just "{escapedPattern}" for deprecated v8 string-selector
// matchers — which keep the v8 wire shape for backward compatibility.
func (s *Subscriber) recomputeEscapedMatchers() {
	s.EscapedMatchers = make([]string, len(s.SubscribedMatchers))
	for i, m := range s.SubscribedMatchers {
		if m.Type == deprecatedMatcherTypeName {
			s.EscapedMatchers[i] = escapeSubscriptionSegment(m.Pattern)
		} else {
			s.EscapedMatchers[i] = escapeSubscriptionSegment(string(m.Type)) + "/" + escapeSubscriptionSegment(m.Pattern)
		}
	}
}

// escapeSubscriptionSegment encodes one path segment of a subscription URL.
// The output contains only RFC 3986 unreserved characters and %XX sequences,
// which (a) is valid in any URL path segment, (b) round-trips through
// url.PathUnescape — used on the receiving side because it tolerates the
// literal '+' that a client may emit when constructing a subscription URL
// per RFC 3986 path rules — and (c) matches a URI Template `{var}`
// expression, keeping the v8 subscription-events URI-template path working.
//
// url.QueryEscape encodes every reserved char except space (which it turns
// into '+'). Replacing the resulting '+' with %20 closes that gap without
// pulling in a hand-rolled escaper.
func escapeSubscriptionSegment(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

// resolveSubscriptionPayloads fills SubscriptionPayloads following the
// spec rule: "the payload value of the first topic matcher in the
// mercure.subscribe claim that matches the subscription's own matcher,
// falling back to mercure.payload". A claim "matches" when its matcher
// accepts the subscription's pattern as a topic, or when the claim is the
// wildcard `*`.
func (s *Subscriber) resolveSubscriptionPayloads() {
	if len(s.SubscribedMatchers) == 0 {
		s.SubscriptionPayloads = nil

		return
	}

	s.SubscriptionPayloads = make([]any, len(s.SubscribedMatchers))

	// Fast path: when no authorization detail carries a payload, every matcher
	// resolves to the same token-wide fallback, so skip the per-matcher matcher
	// dispatch — which at the protocol caps (100 matchers × 100 detail topics)
	// would otherwise run thousands of URL Pattern evaluations per request.
	var authz *mercureAuthz
	if s.Claims != nil {
		authz = s.Claims.authz
	}

	if !authz.hasPayload() {
		fallback := s.legacyPayloadFallback()
		for i := range s.SubscriptionPayloads {
			s.SubscriptionPayloads[i] = fallback
		}

		return
	}

	for i, m := range s.SubscribedMatchers {
		s.SubscriptionPayloads[i] = s.resolveSubscriptionPayload(m)
	}
}

func (s *Subscriber) resolveSubscriptionPayload(m TopicMatcher) any {
	if s.Claims == nil {
		return nil
	}

	// The payload of the first subscribe authorization detail whose topics
	// match the subscription's matcher wins. A matching detail with no payload
	// falls through to the legacy mercure.payload fallback (compatibility
	// mode only; nil otherwise).
	if p, ok := s.Claims.authz.subscribePayload(s.topicSelectorStore, m); ok && p != nil {
		return p
	}

	return s.legacyPayloadFallback()
}

// getSubscriptions returns the subscriptions associated to this subscriber,
// optionally filtered by path variables from the subscription API. A filter
// with neither topic nor match set is treated as "no filter".
func (s *Subscriber) getSubscriptions(filter subscriptionFilter, active bool) []subscription {
	useMatch := filter.match != "" || filter.match_type != ""

	var subscriptions []subscription //nolint:prealloc

	for k, m := range s.SubscribedMatchers {
		switch {
		case useMatch:
			if filter.match != m.Pattern || filter.match_type != string(m.Type) {
				continue
			}
		case filter.topic != "":
			// The deprecated /subscriptions/{topic}[/{subscriber}] route
			// is addressable only by v8 string-selector subscriptions;
			// modern subscriptions live exclusively under
			// /subscriptions/{match_type}/{match}.
			if m.Type != deprecatedMatcherTypeName || filter.topic != m.Pattern {
				continue
			}
		}

		sub := subscription{
			ID:         "/.well-known/mercure/subscriptions/" + s.EscapedMatchers[k] + "/" + s.EscapedID,
			Type:       "subscription",
			Subscriber: s.ID,
			Active:     active,
		}

		// Deprecated v8 subscriptions keep emitting the `topic` field (and
		// no match/match_type) for wire compatibility with v8 consumers.
		if m.Type == deprecatedMatcherTypeName {
			sub.Topic = m.Pattern
		} else {
			sub.Match = m.Pattern
			sub.MatchType = string(m.Type)
		}

		if k < len(s.SubscriptionPayloads) {
			sub.Payload = s.SubscriptionPayloads[k]
		}

		subscriptions = append(subscriptions, sub)
	}

	return subscriptions
}
