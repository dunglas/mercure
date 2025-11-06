package mercure

import (
	"log/slog"
	"net/url"
	"regexp"
)

// Subscriber represents a client subscribed to a list of topics on a remote or on the current hub.
type Subscriber struct {
	ID                     string
	EscapedID              string
	Claims                 *claims
	EscapedTopics          []string
	RequestLastEventID     string
	SubscribedTopics       []string
	SubscribedTopicRegexps []*regexp.Regexp
	AllowedPrivateTopics   []string
	AllowedPrivateRegexps  []*regexp.Regexp

	logger             *slog.Logger
	topicSelectorStore *TopicSelectorStore
}

func NewSubscriber(logger *slog.Logger, topicSelectorStore *TopicSelectorStore) *Subscriber {
	return &Subscriber{
		logger:             logger,
		topicSelectorStore: topicSelectorStore,
	}
}

// SetTopics compiles topic selector regexps.
func (s *Subscriber) SetTopics(subscribedTopics, allowedPrivateTopics []string) {
	s.SubscribedTopics = subscribedTopics
	s.AllowedPrivateTopics = allowedPrivateTopics
	s.EscapedTopics = escapeTopics(subscribedTopics)
}

func escapeTopics(topics []string) []string {
	escapedTopics := make([]string, 0, len(topics))
	for _, topic := range topics {
		escapedTopics = append(escapedTopics, url.QueryEscape(topic))
	}

	return escapedTopics
}

// MatchTopics checks if the current subscriber can access to at least one of the given topics.
//
//nolint:gocognit
func (s *Subscriber) MatchTopics(topics []string, private bool) bool {
	var subscribed bool

	canAccess := !private

	for _, topic := range topics {
		if !subscribed {
			for _, ts := range s.SubscribedTopics {
				if s.topicSelectorStore.match(topic, ts) {
					subscribed = true

					break
				}
			}
		}

		if !canAccess {
			for _, ts := range s.AllowedPrivateTopics {
				if s.topicSelectorStore.match(topic, ts) {
					canAccess = true

					break
				}
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

	if s.AllowedPrivateTopics != nil {
		attrs = append(attrs, slog.Any("topic_selectors", s.AllowedPrivateTopics))
	}

	if s.SubscribedTopics != nil {
		attrs = append(attrs, slog.Any("topics", s.SubscribedTopics))
	}

	return slog.GroupValue(attrs...)
}

// getSubscriptions return the list of subscriptions associated to this subscriber.
func (s *Subscriber) getSubscriptions(topic, context string, active bool) []subscription {
	var subscriptions []subscription //nolint:prealloc

	for k, t := range s.SubscribedTopics {
		if topic != "" && (!s.MatchTopics([]string{topic}, false) || t != topic) {
			continue
		}

		subscription := subscription{
			Context:    context,
			ID:         "/.well-known/mercure/subscriptions/" + s.EscapedTopics[k] + "/" + s.EscapedID,
			Type:       "Subscription",
			Subscriber: s.ID,
			Topic:      t,
			Active:     active,
		}
		if s.Claims != nil && s.Claims.Mercure.Payload != nil {
			subscription.Payload = s.Claims.Mercure.Payload
		}

		subscriptions = append(subscriptions, subscription)
	}

	return subscriptions
}
