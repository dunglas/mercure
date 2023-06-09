package mercure

import (
	"errors"
	"fmt"
	"net/url"
	"sync"
)

// EarliestLastEventID is the reserved value representing the earliest available event id.
const EarliestLastEventID = "earliest"

// TransportFactory is the factory to initialize a new transport.
type TransportFactory = func(u *url.URL, l Logger) (Transport, error)

var (
	transportFactories   = make(map[string]TransportFactory) //nolint:gochecknoglobals
	transportFactoriesMu sync.RWMutex                        //nolint:gochecknoglobals
)

func RegisterTransportFactory(scheme string, factory TransportFactory) {
	transportFactoriesMu.Lock()
	transportFactories[scheme] = factory
	transportFactoriesMu.Unlock()
}

func NewTransport(u *url.URL, l Logger) (Transport, error) { //nolint:ireturn
	transportFactoriesMu.RLock()
	f, ok := transportFactories[u.Scheme]
	transportFactoriesMu.RUnlock()

	if !ok {
		return nil, &TransportError{dsn: u.Redacted(), msg: "no such transport available"}
	}

	return f(u, l)
}

// Transport provides methods to dispatch and persist updates.
type Transport interface {
	// Dispatch dispatches an update to all subscribers.
	Dispatch(update *Update) error

	// AddSubscriber adds a new subscriber to the transport.
	AddSubscriber(s *Subscriber) error

	// RemoveSubscriber removes a subscriber from the transport.
	RemoveSubscriber(s *Subscriber) error

	// Close closes the Transport.
	Close() error
}

// TransportSubscribers provide a method to retrieve the list of active subscribers.
type TransportSubscribers interface {
	// GetSubscribers gets the last event ID and the list of active subscribers at this time.
	GetSubscribers() (string, []*Subscriber, error)
}

// ErrClosedTransport is returned by the Transport's Dispatch and AddSubscriber methods after a call to Close.
var ErrClosedTransport = errors.New("hub: read/write on closed Transport")

// TransportError is returned when the Transport's DSN is invalid.
type TransportError struct {
	dsn string
	msg string
	err error
}

func (e *TransportError) Error() string {
	if e.msg == "" {
		if e.err == nil {
			return fmt.Sprintf("%q: invalid transport", e.dsn)
		}

		return fmt.Sprintf("%q: invalid transport: %s", e.dsn, e.err)
	}

	if e.err == nil {
		return fmt.Sprintf("%q: invalid transport: %s", e.dsn, e.msg)
	}

	return fmt.Sprintf("%q: %s: invalid transport: %s", e.dsn, e.msg, e.err)
}

func (e *TransportError) Unwrap() error {
	return e.err
}

func getSubscribers(sl *SubscriberList) (subscribers []*Subscriber) {
	sl.Walk(0, func(s *Subscriber) bool {
		subscribers = append(subscribers, s)

		return true
	})

	return
}
