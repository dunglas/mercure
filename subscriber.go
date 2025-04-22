package mercure

import (
	"fmt"
	"net/url"
	"regexp"

	"go.uber.org/zap/zapcore"
)

// Subscriber represents a client subscribed to a list of topics on a remote or on the current hub.
type Subscriber struct {
	ID                     string
	EscapedID              string
	Claims                 *claims
	EscapedTopics          []string
	RequestLastEventID     string
	RemoteAddr             string
	SubscribedTopics       []string
	SubscribedTopicRegexps []*regexp.Regexp
	AllowedPrivateTopics   []string
	AllowedPrivateRegexps  []*regexp.Regexp

	logger             Logger
	topicSelectorStore *TopicSelectorStore
}

func NewSubscriber(logger Logger, topicSelectorStore *TopicSelectorStore) *Subscriber {
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

// MatchTopics checks if the current subscriber can access to the given topic.
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

func (s *Subscriber) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("id", s.ID)
	enc.AddString("last_event_id", s.RequestLastEventID)
	if s.RemoteAddr != "" {
		enc.AddString("remote_addr", s.RemoteAddr)
	}
	if s.AllowedPrivateTopics != nil {
		if err := enc.AddArray("topic_selectors", stringArray(s.AllowedPrivateTopics)); err != nil {
			return fmt.Errorf("log error: %w", err)
		}
	}
	if s.SubscribedTopics != nil {
		if err := enc.AddArray("topics", stringArray(s.SubscribedTopics)); err != nil {
			return fmt.Errorf("log error: %w", err)
		}
	}

	return nil
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
