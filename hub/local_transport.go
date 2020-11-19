package hub

import (
	"net/url"
	"sync"
)

func init() {
	RegisterTransportFactory("local", newLocalTransport)
}

// localTransport implements the TransportInterface without database and simply broadcast the live Updates.
type localTransport struct {
	sync.RWMutex
	subscribers map[*Subscriber]struct{}
	lastEventID string
	closed      chan struct{}
	closedOnce  sync.Once
}

// newLocalTransport create a new LocalTransport.
func newLocalTransport(u *url.URL, l Logger) (Transport, error) {
	return &localTransport{
		subscribers: make(map[*Subscriber]struct{}),
		closed:      make(chan struct{}),
		lastEventID: EarliestLastEventID,
	}, nil
}

// Dispatch dispatches an update to all subscribers.
func (t *localTransport) Dispatch(update *Update) error {
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
func (t *localTransport) AddSubscriber(s *Subscriber) error {
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
func (t *localTransport) GetSubscribers() (lastEventID string, subscribers []*Subscriber) {
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
func (t *localTransport) Close() (err error) {
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
