package mercure

import (
	"fmt"
	"net/url"
	"regexp"
	"sync"
	"sync/atomic"

	"github.com/gofrs/uuid"
	uritemplate "github.com/yosida95/uritemplate/v3"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Subscriber represents a client subscribed to a list of topics.
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
	Debug                  bool

	disconnected        int32
	out                 chan *Update
	outMutex            sync.RWMutex
	responseLastEventID chan string
	logger              Logger
	ready               int32
	liveQueue           []*Update
	liveMutex           sync.RWMutex
}

const outBufferLength = 1000

// NewSubscriber creates a new subscriber.
func NewSubscriber(lastEventID string, logger Logger) *Subscriber {
	id := "urn:uuid:" + uuid.Must(uuid.NewV4()).String()
	s := &Subscriber{
		ID:                  id,
		EscapedID:           url.QueryEscape(id),
		RequestLastEventID:  lastEventID,
		responseLastEventID: make(chan string, 1),
		out:                 make(chan *Update, outBufferLength),
		logger:              logger,
	}

	return s
}

// Dispatch an update to the subscriber.
// Security checks must (topics matching) be done before calling Dispatch,
// for instance by calling Match.
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
	if atomic.LoadInt32(&s.disconnected) > 0 {
		s.outMutex.Unlock()

		return false
	}

	select {
	case s.out <- u:
		s.outMutex.Unlock()
	default:
		s.handleFullChan()

		return false
	}

	return true
}

// Ready flips the ready flag to true and flushes queued live updates returning number of events flushed.
func (s *Subscriber) Ready() (n int) {
	s.liveMutex.Lock()
	s.outMutex.Lock()

	for _, u := range s.liveQueue {
		select {
		case s.out <- u:
			n++
		default:
			s.handleFullChan()
			s.liveMutex.Unlock()

			return n
		}
	}
	atomic.StoreInt32(&s.ready, 1)

	s.outMutex.Unlock()
	s.liveMutex.Unlock()

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
func (s *Subscriber) SetTopics(subscribedTopics, allowedPrivateTopics []string) {
	s.SubscribedTopics = subscribedTopics
	s.SubscribedTopicRegexps = make([]*regexp.Regexp, len(subscribedTopics))
	for i, ts := range subscribedTopics {
		var r *regexp.Regexp
		if tpl, err := uritemplate.New(ts); err == nil {
			r = tpl.Regexp()
		}
		s.SubscribedTopicRegexps[i] = r
	}
	s.AllowedPrivateTopics = allowedPrivateTopics
	s.AllowedPrivateRegexps = make([]*regexp.Regexp, len(allowedPrivateTopics))
	for i, ts := range allowedPrivateTopics {
		var r *regexp.Regexp
		if tpl, err := uritemplate.New(ts); err == nil {
			r = tpl.Regexp()
		}
		s.AllowedPrivateRegexps[i] = r
	}
	s.EscapedTopics = escapeTopics(subscribedTopics)
}

func escapeTopics(topics []string) []string {
	escapedTopics := make([]string, 0, len(topics))
	for _, topic := range topics {
		escapedTopics = append(escapedTopics, url.QueryEscape(topic))
	}

	return escapedTopics
}

// MatchTopic checks if the current subscriber can access to the given topic.
//
//nolint:gocognit
func (s *Subscriber) MatchTopics(topics []string, private bool) bool {
	var subscribed bool
	canAccess := !private

	for _, topic := range topics {
		if !subscribed {
			for i, ts := range s.SubscribedTopics {
				if ts == "*" || ts == topic {
					subscribed = true

					break
				}

				r := s.SubscribedTopicRegexps[i]
				if r != nil && r.MatchString(topic) {
					subscribed = true

					break
				}
			}
		}

		if !canAccess {
			for i, ts := range s.AllowedPrivateTopics {
				if ts == "*" || ts == topic {
					canAccess = true

					break
				}

				r := s.AllowedPrivateRegexps[i]
				if r != nil && r.MatchString(topic) {
					canAccess = true

					break
				}
			}
		}

		if subscribed && canAccess {
			return true
		}
	}

	return false
}

// Match checks if the current subscriber can receive the given update.
func (s *Subscriber) Match(u *Update) bool {
	return s.MatchTopics(u.Topics, u.Private)
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

// handleFullChan disconnects the subscriber when the out channel is full.
func (s *Subscriber) handleFullChan() {
	atomic.StoreInt32(&s.disconnected, 1)
	s.outMutex.Unlock()

	if c := s.logger.Check(zap.ErrorLevel, "subscriber unable to receive updates fast enough"); c != nil {
		c.Write(zap.Object("subscriber", s))
	}
}
