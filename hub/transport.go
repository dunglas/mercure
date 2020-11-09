package hub

import (
	"errors"
	"fmt"
	"net/url"
	"sync"

	"github.com/spf13/viper"
)

// EarliestLastEventID is the reserved value representing the earliest available event id.
const EarliestLastEventID = "earliest"

// Transport provides methods to dispatch and persist updates.
type Transport interface {
	// Dispatch dispatches an update to all subscribers.
	Dispatch(update *Update) error

	// AddSubscriber adds a new subscriber to the transport.
	AddSubscriber(s *Subscriber) error

	// Close closes the Transport.
	Close() error
}

// TransportSubscribers provide a method to retrieve the list of active subscribers.
type TransportSubscribers interface {
	// GetSubscribers gets the last event ID and the list of active subscribers at this time.
	GetSubscribers() (string, []*Subscriber)
}

// ErrClosedTransport is returned by the Transport's Dispatch and AddSubscriber methods after a call to Close.
var ErrClosedTransport = errors.New("hub: read/write on closed Transport")

// ErrInvalidTransportDSN is returned when the Transport's DSN is invalid.
type ErrInvalidTransportDSN struct {
	dsn string
	msg string
	err error
}

func (e *ErrInvalidTransportDSN) Error() string {
	if e.msg == "" {
		if e.err == nil {
			return fmt.Sprintf("%q: invalid transport DSN", e.dsn)
		}

		return fmt.Sprintf("%q: %s: invalid transport DSN", e.dsn, e.err)
	}

	if e.err == nil {
		return fmt.Sprintf("%q: %s: invalid transport DSN", e.dsn, e.msg)
	}

	return fmt.Sprintf("%q: %s: %s: invalid transport DSN", e.dsn, e.msg, e.err)
}

func (e *ErrInvalidTransportDSN) Unwrap() error {
	return e.err
}

// NewTransport create a transport using the backend matching the given TransportURL.
func NewTransport(config *viper.Viper, logger Logger) (Transport, error) {
	tu := config.GetString("transport_url")
	if tu == "" {
		return NewLocalTransport(), nil
	}

	u, err := url.Parse(tu)
	if err != nil {
		return nil, fmt.Errorf("transport_url: %w", err)
	}

	switch u.Scheme {
	case "null":
		return NewLocalTransport(), nil

	case "bolt":
		return NewBoltTransport(u, logger)
	}

	return nil, &ErrInvalidTransportDSN{dsn: tu, msg: "no such transport available"}
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
func NewLocalTransport() *LocalTransport {
	return &LocalTransport{
		subscribers: make(map[*Subscriber]struct{}),
		closed:      make(chan struct{}),
		lastEventID: EarliestLastEventID,
	}
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
