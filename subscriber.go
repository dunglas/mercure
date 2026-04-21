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
	var subscribed bool

	canAccess := !private

	for _, m := range s.SubscribedMatchers {
		if !subscribed && s.topicSelectorStore.matchMatcher(topics, m) {
			subscribed = true

			if canAccess {
				return true
			}
		}
	}

	if !canAccess {
		for _, m := range s.AllowedPrivateMatchers {
			if s.topicSelectorStore.matchMatcher(topics, m) {
				canAccess = true

				if subscribed {
					return true
				}

				break
			}
		}
	}

	return subscribed && canAccess
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

// setMatchers sets the subscribed and allowed private topic matchers.
func (s *Subscriber) setMatchers(subscribed []topicMatcher, allowedPrivate []topicMatcher) {
	s.SubscribedMatchers = subscribed
	s.AllowedPrivateMatchers = allowedPrivate

	s.EscapedMatchers = make([]string, len(subscribed))
	for i, m := range subscribed {
		s.EscapedMatchers[i] = url.QueryEscape(m.Type) + "/" + url.QueryEscape(m.Pattern)
	}
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
			Match:      m.Pattern,
			MatchType:  m.Type,
			Active:     active,
		}

		// Deprecated v8 subscriptions keep emitting the `topic` field (and
		// no match/matchType) for wire compatibility with v8 consumers.
		if m.Type == deprecatedMatcherTypeName {
			sub.ID = "/.well-known/mercure/subscriptions/" + url.QueryEscape(m.Pattern) + "/" + s.EscapedID
			sub.Topic = m.Pattern
			sub.Match = ""
			sub.MatchType = ""
		}

		s.setSubscriptionPayload(&sub, m)
		subscriptions = append(subscriptions, sub)
	}

	return subscriptions
}

// setSubscriptionPayload attaches the subscription payload following the spec rule:
// "The payload value associated with the first topic matcher in the mercure.subscribe
// claim that matches the subscription's own matcher."
//
// A claim "matches" the subscription when its matcher accepts the subscription's
// pattern as a topic or when the wildcard `*` is used. When no claim matches,
// the top-level mercure.payload fallback is applied.
func (s *Subscriber) setSubscriptionPayload(sub *subscription, m topicMatcher) {
	if s.Claims == nil {
		return
	}

	for _, mc := range s.Claims.Mercure.Subscribe {
		if mc.Pattern != "*" && !s.topicSelectorStore.matchMatcher([]string{m.Pattern}, mc.topicMatcher) {
			continue
		}

		if mc.Payload != nil {
			sub.Payload = mc.Payload

			return
		}

		break // first matching claim wins, even if it has no per-claim payload
	}

	// Global payload fallback
	if s.Claims.Mercure.Payload != nil {
		sub.Payload = s.Claims.Mercure.Payload
	}
}
