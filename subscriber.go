package mercure

import (
	"net/url"
	"sync"

	"github.com/gofrs/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type updateSource struct {
	in     chan *Update
	buffer []*Update
}

// Subscriber represents a client subscribed to a list of topics.
type Subscriber struct {
	ID                 string
	EscapedID          string
	Claims             *claims
	Topics             []string
	EscapedTopics      []string
	RequestLastEventID string
	RemoteAddr         string
	TopicSelectors     []string
	Debug              bool

	disconnectedOnce    sync.Once
	out                 chan *Update
	disconnected        chan struct{}
	responseLastEventID chan string
	history             updateSource
	live                updateSource
	logger              Logger
	topicSelectorStore  *TopicSelectorStore
}

// NewSubscriber creates a new subscriber.
func NewSubscriber(lastEventID string, logger Logger, tss *TopicSelectorStore) *Subscriber {
	id := "urn:uuid:" + uuid.Must(uuid.NewV4()).String()
	s := &Subscriber{
		ID:                 id,
		EscapedID:          url.QueryEscape(id),
		RequestLastEventID: lastEventID,

		responseLastEventID: make(chan string, 1),

		history:            updateSource{},
		live:               updateSource{in: make(chan *Update)},
		out:                make(chan *Update),
		disconnected:       make(chan struct{}),
		logger:             logger,
		topicSelectorStore: tss,
	}

	if lastEventID != "" {
		s.history.in = make(chan *Update)
	}

	return s
}

// start stores incoming updates in an history and a live buffer and dispatch them.
// Updates coming from the history are always dispatched first.
func (s *Subscriber) start() {
	for {
		select {
		case u, ok := <-s.history.in:
			if !ok {
				s.history.in = nil

				break
			}
			if s.CanDispatch(u) {
				s.history.buffer = append(s.history.buffer, u)
			}
		case u, ok := <-s.live.in:
			if !ok {
				// chan drained
				return
			}
			if s.CanDispatch(u) {
				s.live.buffer = append(s.live.buffer, u)
			}
		case s.outChan() <- s.nextUpdate():
			if len(s.history.buffer) > 0 {
				s.history.buffer = s.history.buffer[1:]

				break
			}

			s.live.buffer = s.live.buffer[1:]
		}
	}
}

// outChan returns the out channel if buffers aren't empty, or nil to block.
func (s *Subscriber) outChan() chan<- *Update {
	if len(s.live.buffer) > 0 || len(s.history.buffer) > 0 {
		return s.out
	}

	return nil
}

// nextUpdate returns the next update to dispatch.
// The history is always entirely flushed before starting to dispatch live updates.
func (s *Subscriber) nextUpdate() *Update {
	// Always flush the history buffer first to preserve order
	if s.history.in != nil || len(s.history.buffer) > 0 {
		if len(s.history.buffer) > 0 {
			return s.history.buffer[0]
		}

		return nil
	}

	if len(s.live.buffer) > 0 {
		return s.live.buffer[0]
	}

	return nil
}

// Dispatch an update to the subscriber.
func (s *Subscriber) Dispatch(u *Update, fromHistory bool) bool {
	var in chan<- *Update
	if fromHistory {
		in = s.history.in
	} else {
		in = s.live.in
	}

	select {
	case <-s.disconnected:
		close(s.live.in)

		return false

	default:
	}

	select {
	case <-s.disconnected:
		close(s.live.in)

		return false

	case in <- u:
	}

	return true
}

// Receive returns a chan when incoming updates are dispatched.
func (s *Subscriber) Receive() <-chan *Update {
	return s.out
}

// HistoryDispatched must be called when all messages coming from the history have been dispatched.
func (s *Subscriber) HistoryDispatched(responseLastEventID string) {
	s.responseLastEventID <- responseLastEventID
	close(s.history.in)
}

// Disconnect disconnects the subscriber.
func (s *Subscriber) Disconnect() {
	s.disconnectedOnce.Do(func() {
		close(s.disconnected)
	})
}

// CanDispatch checks if an update can be dispatched to this subsriber.
func (s *Subscriber) CanDispatch(u *Update) bool {
	if !canReceive(s.topicSelectorStore, u.Topics, s.Topics) {
		s.logger.Debug("Subscriber has not subscribed to this update", zap.Object("subscriber", s), zap.Object("update", u))

		return false
	}

	if u.Private && (s.Claims == nil || s.Claims.Mercure.Subscribe == nil || !canReceive(s.topicSelectorStore, u.Topics, s.Claims.Mercure.Subscribe)) {
		s.logger.Debug("Subscriber not authorized to receive this update", zap.Object("subscriber", s), zap.Object("update", u))

		return false
	}

	return true
}

// getSubscriptions return the list of subscriptions associated to this subscriber.
func (s *Subscriber) getSubscriptions(topic, context string, active bool) []subscription {
	subscriptions := make([]subscription, 0, len(s.Topics))

	for k, t := range s.Topics {
		if topic != "" && !canReceive(s.topicSelectorStore, []string{t}, []string{topic}) {
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
	if s.TopicSelectors != nil {
		enc.AddArray("topic_selectors", stringArray(s.TopicSelectors))
	}
	if s.Topics != nil {
		enc.AddArray("topics", stringArray(s.Topics))
	}

	return nil
}
