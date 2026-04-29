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

	// SubscribedMatchers are the topic matchers from match* query parameters
	// (or from the deprecated `topic` query parameter, which resolves to a
	// deprecatedMatcher-backed topicMatcher).
	SubscribedMatchers []topicMatcher
	// AllowedPrivateMatchers are the topic matchers from the JWT claims.
	AllowedPrivateMatchers []topicMatcher
	// EscapedMatchers are precomputed "escapedType/escapedPattern" slugs for
	// subscription URLs.
	EscapedMatchers []string
	// SubscriptionPayloads holds the JSON-LD `payload` resolved per
	// subscribed matcher at registration time, indexed parallel to
	// SubscribedMatchers. Precomputing lets the subscription API render
	// payloads for subscribers a transport reconstructed from its
	// persistence layer (Redis, …) without doing live matcher dispatch
	// on the deserialized object.
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

func (s *Subscriber) matchesAny(topics []string, matchers []topicMatcher) bool {
	for _, m := range matchers {
		if s.topicSelectorStore.matchMatcher(topics, m) {
			return true
		}
	}

	return false
}

// Match checks if the current subscriber can receive the given update.
func (s *Subscriber) Match(u *Update) bool {
	return s.MatchTopics(u.Topics, u.Private)
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

func logMatcherPatterns(matchers []topicMatcher) []string {
	out := make([]string, len(matchers))
	for i, m := range matchers {
		out[i] = m.Type + ":" + m.Pattern
	}

	return out
}

// setMatchers sets the subscribed and allowed private topic matchers, and
// precomputes the per-matcher subscription payload so the subscription
// API can render it from a serialized Subscriber without re-running the
// matcher dispatch.
func (s *Subscriber) setMatchers(subscribed []topicMatcher, allowedPrivate []topicMatcher) {
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
			s.EscapedMatchers[i] = url.QueryEscape(m.Pattern)
		} else {
			s.EscapedMatchers[i] = url.QueryEscape(m.Type) + "/" + url.QueryEscape(m.Pattern)
		}
	}
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
	for i, m := range s.SubscribedMatchers {
		s.SubscriptionPayloads[i] = s.resolveSubscriptionPayload(m)
	}
}

func (s *Subscriber) resolveSubscriptionPayload(m topicMatcher) any {
	if s.Claims == nil {
		return nil
	}

	for _, mc := range s.Claims.Mercure.Subscribe {
		if mc.Pattern != "*" && !s.topicSelectorStore.matchMatcher([]string{m.Pattern}, mc.topicMatcher) {
			continue
		}

		if mc.Payload != nil {
			return mc.Payload
		}

		break // first matching claim wins, even if it has no per-claim payload
	}

	return s.Claims.Mercure.Payload
}

// getSubscriptions returns the subscriptions associated to this subscriber,
// optionally filtered by path variables from the subscription API. A filter
// with neither topic nor match set is treated as "no filter".
func (s *Subscriber) getSubscriptions(filter subscriptionFilter, context string, active bool) []subscription {
	useMatch := filter.match != "" || filter.matchType != ""

	var subscriptions []subscription //nolint:prealloc

	for k, m := range s.SubscribedMatchers {
		switch {
		case useMatch:
			if filter.match != m.Pattern || !strings.EqualFold(filter.matchType, m.Type) {
				continue
			}
		case filter.topic != "":
			// The deprecated /subscriptions/{topic}[/{subscriber}] route
			// is addressable only by v8 string-selector subscriptions;
			// modern match*-based subscriptions live exclusively under
			// /subscriptions/{matchType}/{match}.
			if m.Type != deprecatedMatcherTypeName || filter.topic != m.Pattern {
				continue
			}
		}

		sub := subscription{
			Context:    context,
			ID:         "/.well-known/mercure/subscriptions/" + s.EscapedMatchers[k] + "/" + s.EscapedID,
			Type:       "Subscription",
			Subscriber: s.ID,
			Active:     active,
		}

		// Deprecated v8 subscriptions keep emitting the `topic` field (and
		// no match/matchType) for wire compatibility with v8 consumers.
		if m.Type == deprecatedMatcherTypeName {
			sub.Topic = m.Pattern
		} else {
			sub.Match = m.Pattern
			sub.MatchType = m.Type
		}

		if k < len(s.SubscriptionPayloads) {
			sub.Payload = s.SubscriptionPayloads[k]
		}

		subscriptions = append(subscriptions, sub)
	}

	return subscriptions
}
