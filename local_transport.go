package mercure

import (
	"net/url"
	"sync"
)

func init() { //nolint:gochecknoinits
	RegisterTransportFactory("local", NewLocalTransport)
}

// LocalTransport implements the TransportInterface without database and simply broadcast the live Updates.
type LocalTransport struct {
	sync.RWMutex
	subscribers *SubscriberList
	lastEventID string
	closed      chan struct{}
	closedOnce  sync.Once
}

// NewLocalTransport create a new LocalTransport.
func NewLocalTransport(_ *url.URL, _ Logger) (Transport, error) { //nolint:ireturn
	return &LocalTransport{
		subscribers: NewSubscriberList(1e5),
		closed:      make(chan struct{}),
		lastEventID: EarliestLastEventID,
	}, nil
}

// Dispatch dispatches an update to all subscribers.
func (t *LocalTransport) Dispatch(update *Update) error {
	select {
	case <-t.closed:
		return ErrClosedTransport
	default:
	}

	AssignUUID(update)
	for _, s := range t.subscribers.MatchAny(update) {
		s.Dispatch(update, false)
	}
	t.Lock()
	t.lastEventID = update.ID
	t.Unlock()

	return nil
}

// AddSubscriber adds a new subscriber to the transport.
func (t *LocalTransport) AddSubscriber(s *Subscriber) error {
	select {
	case <-t.closed:
		return ErrClosedTransport
	default:
	}

	t.Lock()
	defer t.Unlock()

	t.subscribers.Add(s)
	if s.RequestLastEventID != "" {
		s.HistoryDispatched(EarliestLastEventID)
	}
	s.Ready()

	return nil
}

// RemoveSubscriber removes a subscriber from the transport.
func (t *LocalTransport) RemoveSubscriber(s *Subscriber) error {
	select {
	case <-t.closed:
		return ErrClosedTransport
	default:
	}

	t.Lock()
	defer t.Unlock()
	t.subscribers.Remove(s)

	return nil
}

// GetSubscribers gets the list of active subscribers.
func (t *LocalTransport) GetSubscribers() (string, []*Subscriber, error) {
	t.RLock()
	defer t.RUnlock()

	return t.lastEventID, getSubscribers(t.subscribers), nil
}

// Close closes the Transport.
func (t *LocalTransport) Close() (err error) {
	t.closedOnce.Do(func() {
		t.Lock()
		defer t.Unlock()
		close(t.closed)
		t.subscribers.Walk(0, func(s *Subscriber) bool {
			s.Disconnect()

			return true
		})
	})

	return nil
}

// Interface guard.
var _ Transport = (*LocalTransport)(nil)
