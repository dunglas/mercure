package mercure

import (
	"fmt"
	"net/url"
	"regexp"
	"sync"
	"sync/atomic"

	"github.com/gofrs/uuid"
	uritemplate "github.com/yosida95/uritemplate/v3"
	"go.uber.org/zap/zapcore"
)

// Subscriber represents a client subscribed to a list of topics.
type Subscriber struct {
	ID                 string
	EscapedID          string
	Claims             *claims
	EscapedTopics      []string
	RequestLastEventID string
	RemoteAddr         string
	Topics             []string
	TopicRegexps       []*regexp.Regexp
	PrivateTopics      []string
	PrivateRegexps     []*regexp.Regexp
	Debug              bool

	disconnected        int32
	out                 chan *Update
	outMutex            sync.RWMutex
	responseLastEventID chan string
	logger              Logger
	ready               int32
	liveQueue           []*Update
	liveMutex           sync.RWMutex
}

// NewSubscriber creates a new subscriber.
func NewSubscriber(lastEventID string, logger Logger) *Subscriber {
	id := "urn:uuid:" + uuid.Must(uuid.NewV4()).String()
	s := &Subscriber{
		ID:                  id,
		EscapedID:           url.QueryEscape(id),
		RequestLastEventID:  lastEventID,
		responseLastEventID: make(chan string, 1),
		out:                 make(chan *Update, 1000),
		logger:              logger,
	}

	return s
}

// Dispatch an update to the subscriber.
func (s *Subscriber) Dispatch(u *Update, fromHistory bool) bool {
	if atomic.LoadInt32(&s.disconnected) > 0 {
		return false
	}

	if !fromHistory && atomic.LoadInt32(&s.ready) < 1 {
		s.liveMutex.Lock()
		if s.ready < 1 {
			s.liveQueue = append(s.liveQueue, u)
			s.liveMutex.Unlock()

			return true
		}
		s.liveMutex.Unlock()
	}

	s.outMutex.Lock()
	defer s.outMutex.Unlock()
	if atomic.LoadInt32(&s.disconnected) > 0 {
		return false
	}

	s.out <- u

	return true
}

// Ready flips the ready flag to true and flushes queued live updates returning number of events flushed.
func (s *Subscriber) Ready() int {
	s.liveMutex.Lock()
	defer s.liveMutex.Unlock()
	s.outMutex.Lock()
	defer s.outMutex.Unlock()

	n := len(s.liveQueue)
	for _, u := range s.liveQueue {
		s.out <- u
	}
	atomic.StoreInt32(&s.ready, 1)

	return n
}

// Receive returns a chan when incoming updates are dispatched.
func (s *Subscriber) Receive() <-chan *Update {
	return s.out
}

// HistoryDispatched must be called when all messages coming from the history have been dispatched.
func (s *Subscriber) HistoryDispatched(responseLastEventID string) {
	s.responseLastEventID <- responseLastEventID
}

// Disconnect disconnects the subscriber.
func (s *Subscriber) Disconnect() {
	if atomic.LoadInt32(&s.disconnected) > 0 {
		return
	}

	s.outMutex.Lock()
	defer s.outMutex.Unlock()

	atomic.StoreInt32(&s.disconnected, 1)
	close(s.out)
}

// SetTopics compiles topic selector regexps.
func (s *Subscriber) SetTopics(topics, privateTopics []string) {
	s.Topics = topics
	s.TopicRegexps = make([]*regexp.Regexp, len(topics))
	for i, ts := range topics {
		var r *regexp.Regexp
		if tpl, err := uritemplate.New(ts); err == nil {
			r = tpl.Regexp()
		}
		s.TopicRegexps[i] = r
	}
	s.PrivateTopics = privateTopics
	s.PrivateRegexps = make([]*regexp.Regexp, len(privateTopics))
	for i, ts := range privateTopics {
		var r *regexp.Regexp
		if tpl, err := uritemplate.New(ts); err == nil {
			r = tpl.Regexp()
		}
		s.PrivateRegexps[i] = r
	}
	s.EscapedTopics = escapeTopics(topics)
}

func escapeTopics(topics []string) []string {
	escapedTopics := make([]string, 0, len(topics))
	for _, topic := range topics {
		escapedTopics = append(escapedTopics, url.QueryEscape(topic))
	}

	return escapedTopics
}

// Match checks if the current subscriber can access to the given topic.
func (s *Subscriber) Match(topic string, private bool) (match bool) {
	for i, ts := range s.Topics {
		if ts == "*" || ts == topic {
			match = true

			break
		}

		r := s.TopicRegexps[i]
		if r != nil && r.MatchString(topic) {
			match = true

			break
		}
	}

	if !match {
		return false
	}
	if !private {
		return true
	}

	for i, ts := range s.PrivateTopics {
		if ts == "*" || ts == topic {
			return true
		}

		r := s.PrivateRegexps[i]
		if r != nil && r.MatchString(topic) {
			return true
		}
	}

	return false
}

// getSubscriptions return the list of subscriptions associated to this subscriber.
func (s *Subscriber) getSubscriptions(topic, context string, active bool) []subscription {
	var subscriptions []subscription //nolint:prealloc
	for k, t := range s.Topics {
		if topic != "" && !s.Match(topic, false) {
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

func (s *Subscriber) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("id", s.ID)
	enc.AddString("last_event_id", s.RequestLastEventID)
	if s.RemoteAddr != "" {
		enc.AddString("remote_addr", s.RemoteAddr)
	}
	if s.PrivateTopics != nil {
		if err := enc.AddArray("topic_selectors", stringArray(s.PrivateTopics)); err != nil {
			return fmt.Errorf("log error: %w", err)
		}
	}
	if s.Topics != nil {
		if err := enc.AddArray("topics", stringArray(s.Topics)); err != nil {
			return fmt.Errorf("log error: %w", err)
		}
	}

	return nil
}
