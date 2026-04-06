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
	// (or from the legacy `topic` query parameter, which resolves to a
	// legacyMatcher-backed topicMatcher).
	SubscribedMatchers []topicMatcher
	// AllowedPrivateMatchers are the topic matchers from the JWT claims.
	AllowedPrivateMatchers []topicMatcher
	// EscapedMatchers are precomputed URL-safe representations for subscription URLs.
	EscapedMatchers []escapedMatcher

	logger             *slog.Logger
	topicSelectorStore *TopicSelectorStore
}

type escapedMatcher struct {
	EscapedType    string
	EscapedPattern string
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
		attrs = append(attrs, slog.Any("allowed_private_matchers", matcherPatternsLog(s.AllowedPrivateMatchers)))
	}

	if len(s.SubscribedMatchers) != 0 {
		attrs = append(attrs, slog.Any("subscribed_matchers", matcherPatternsLog(s.SubscribedMatchers)))
	}

	return slog.GroupValue(attrs...)
}

// matcherPatternsLog renders a matcher slice as a list of "type:pattern" strings for logs.
func matcherPatternsLog(matchers []topicMatcher) []string {
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

	s.EscapedMatchers = make([]escapedMatcher, len(subscribed))
	for i, m := range subscribed {
		s.EscapedMatchers[i] = escapedMatcher{
			EscapedType:    url.QueryEscape(m.Type),
			EscapedPattern: url.QueryEscape(m.Pattern),
		}
	}
}

// getSubscriptions returns the subscriptions associated to this subscriber, optionally filtered.
// An empty filter returns all subscriptions. When filter.useMatch is true, subscriptions are
// filtered by matcher type+pattern (new URL scheme). Otherwise, filter.topic filters by exact
// legacy topic selector.
func (s *Subscriber) getSubscriptions(filter subscriptionFilter, context string, active bool) []subscription {
	var subscriptions []subscription //nolint:prealloc

	for k, m := range s.SubscribedMatchers {
		switch {
		case filter.useMatch:
			// New URL scheme: filter by exact matchType+match pair.
			if filter.match != m.Pattern || !strings.EqualFold(filter.matchType, m.Type) {
				continue
			}
		case filter.topic != "":
			// Legacy URL scheme against a matcher-based subscription:
			// only an exact-pattern match can be addressed this way.
			if filter.topic != m.Pattern {
				continue
			}
		}

		sub := subscription{
			Context:    context,
			ID:         "/.well-known/mercure/subscriptions/" + s.EscapedMatchers[k].EscapedType + "/" + s.EscapedMatchers[k].EscapedPattern + "/" + s.EscapedID,
			Type:       "Subscription",
			Subscriber: s.ID,
			Match:      m.Pattern,
			MatchType:  m.Type,
			Active:     active,
		}

		// Legacy subscriptions keep emitting the `topic` field (and no
		// match/matchType) for wire compatibility with protocol v8 consumers.
		if m.Type == legacyMatcherTypeName {
			sub.ID = "/.well-known/mercure/subscriptions/" + s.EscapedMatchers[k].EscapedPattern + "/" + s.EscapedID
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
// pattern as a topic, when the wildcard `*` is used, or when both pattern and
// type are identical. When no claim matches, the top-level mercure.payload
// fallback is applied.
func (s *Subscriber) setSubscriptionPayload(sub *subscription, m topicMatcher) {
	if s.Claims == nil {
		return
	}

	for _, mc := range s.Claims.Mercure.Subscribe {
		if !claimMatchesMatcher(s.topicSelectorStore, mc, m) {
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

// claimMatchesMatcher reports whether a JWT subscribe-claim entry "matches" a subscription matcher.
func claimMatchesMatcher(tss *TopicSelectorStore, mc matcherClaim, m topicMatcher) bool {
	// Wildcard always matches.
	if mc.Pattern == "*" {
		return true
	}

	// Same matcher type and same pattern: a trivial match.
	if strings.EqualFold(mc.Type, m.Type) && mc.Pattern == m.Pattern {
		return true
	}

	// Otherwise, apply the claim's matcher against the subscription's pattern string.
	if mc.matcher == nil {
		return false
	}

	return tss.matchMatcher([]string{m.Pattern}, mc.topicMatcher)
}
