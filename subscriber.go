package mercure

import (
	"fmt"
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

// setMatchers sets the subscribed and allowed private topic matchers.
func (s *Subscriber) setMatchers(subscribed []topicMatcher, allowedPrivate []topicMatcher) {
	s.SubscribedMatchers = subscribed
	s.AllowedPrivateMatchers = allowedPrivate
	s.recomputeEscapedMatchers()
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

// BindMatchers resolves the matcher implementation for each entry of
// SubscribedMatchers and AllowedPrivateMatchers against the Subscriber's
// TopicSelectorStore, and rebuilds EscapedMatchers from the result.
//
// Call this after deserializing a Subscriber — for example, when a
// distributed transport restores subscriber state from a persistence
// layer. JSON and gob round-trips drop the unexported matcher binding on
// each topic matcher; without re-binding, subsequent Match and
// subscription-rendering calls would dispatch through a nil interface.
//
// Returns ErrUnsupportedMatcherType if any matcher's Type is not
// registered in the TopicSelectorStore. Already-bound matchers are
// skipped, so the call is idempotent.
func (s *Subscriber) BindMatchers() error {
	for i := range s.SubscribedMatchers {
		if err := s.bindMatcher(&s.SubscribedMatchers[i]); err != nil {
			return err
		}
	}

	for i := range s.AllowedPrivateMatchers {
		if err := s.bindMatcher(&s.AllowedPrivateMatchers[i]); err != nil {
			return err
		}
	}

	s.recomputeEscapedMatchers()

	return nil
}

func (s *Subscriber) bindMatcher(m *topicMatcher) error {
	if m.matcher != nil {
		return nil
	}

	if m.Type == deprecatedMatcherTypeName {
		return bindDeprecatedMatcher(m)
	}

	if s.topicSelectorStore == nil {
		return fmt.Errorf("%w: %s (subscriber has no TopicSelectorStore)", ErrUnsupportedMatcherType, m.Type)
	}

	rm, ok := s.topicSelectorStore.matchers[strings.ToLower(m.Type)]
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnsupportedMatcherType, m.Type)
	}

	m.matcher = rm.matcher

	return nil
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
