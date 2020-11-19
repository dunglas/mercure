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

func newTransport(u *url.URL, l Logger) (Transport, error) {
	transportFactoriesMu.RLock()
	f, ok := transportFactories[u.Scheme]
	transportFactoriesMu.RUnlock()

	if !ok {
		return nil, &ErrInvalidTransportDSN{dsn: u.String(), msg: "no such transport available"}
	}

	return f(u, l)
}

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
