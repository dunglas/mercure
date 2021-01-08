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
	subscribers map[*Subscriber]struct{}
	lastEventID string
	closed      chan struct{}
	closedOnce  sync.Once
}

// NewLocalTransport create a new LocalTransport.
func NewLocalTransport(u *url.URL, l Logger) (Transport, error) {
	return &LocalTransport{
		subscribers: make(map[*Subscriber]struct{}),
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
	t.Lock()
	defer t.Unlock()
	for subscriber := range t.subscribers {
		if !subscriber.Dispatch(update, false) {
			delete(t.subscribers, subscriber)
		}
	}
	t.lastEventID = update.ID

	return nil
}

// AddSubscriber adds a new subscriber to the transport.
func (t *LocalTransport) AddSubscriber(s *Subscriber) error {
	t.Lock()
	defer t.Unlock()

	select {
	case <-t.closed:
		return ErrClosedTransport
	default:
	}

	t.subscribers[s] = struct{}{}
	if s.RequestLastEventID != "" {
		s.HistoryDispatched(EarliestLastEventID)
	}

	return nil
}

// GetSubscribers get the list of active subscribers.
func (t *LocalTransport) GetSubscribers() (lastEventID string, subscribers []*Subscriber) {
	t.RLock()
	defer t.RUnlock()
	subscribers = make([]*Subscriber, len(t.subscribers))

	i := 0
	for subscriber := range t.subscribers {
		subscribers[i] = subscriber
		i++
	}

	return t.lastEventID, subscribers
}

// Close closes the Transport.
func (t *LocalTransport) Close() (err error) {
	t.closedOnce.Do(func() {
		t.Lock()
		defer t.Unlock()
		close(t.closed)
		for subscriber := range t.subscribers {
			subscriber.Disconnect()
			delete(t.subscribers, subscriber)
		}
	})

	return nil
}

// Interface guard.
var _ Transport = (*BoltTransport)(nil)
