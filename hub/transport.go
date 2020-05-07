package hub

import (
	"errors"
	"fmt"
	"net/url"
	"sync"

	"github.com/spf13/viper"
)

// Transport provides methods to dispatch and persist updates.
type Transport interface {
	// Dispatch dispatches an update to all subscribers.
	Dispatch(update *Update) error

	// AddSubscriber adds a new subscriber to the transport.
	AddSubscriber(s *Subscriber) error

	// Close closes the Transport.
	Close() error
}

var (
	// ErrInvalidTransportDSN is returned when the Transport's DSN is invalid
	ErrInvalidTransportDSN = errors.New("invalid transport DSN")
	// ErrClosedTransport is returned by the Transport's Dispatch and AddSubscriber methods after a call to Close.
	ErrClosedTransport = errors.New("hub: read/write on closed Transport")
)

// NewTransport create a transport using the backend matching the given TransportURL.
func NewTransport(config *viper.Viper) (Transport, error) {
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
		return NewBoltTransport(u)
	}

	return nil, fmt.Errorf("%q: no such transport available: %w", tu, ErrInvalidTransportDSN)
}

// LocalTransport implements the TransportInterface without database and simply broadcast the live Updates.
type LocalTransport struct {
	sync.RWMutex
	subscribers map[*Subscriber]struct{}
	done        chan struct{}
}

// NewLocalTransport create a new LocalTransport.
func NewLocalTransport() *LocalTransport {
	return &LocalTransport{
		subscribers: make(map[*Subscriber]struct{}),
		done:        make(chan struct{}),
	}
}

// Dispatch dispatches an update to all subscribers.
func (t *LocalTransport) Dispatch(update *Update) error {
	select {
	case <-t.done:
		return ErrClosedTransport
	default:
	}

	t.Lock()
	defer t.Unlock()
	for subscriber := range t.subscribers {
		if !subscriber.Dispatch(update, false) {
			delete(t.subscribers, subscriber)
		}
	}

	return nil
}

// AddSubscriber adds a new subscriber to the transport.
func (t *LocalTransport) AddSubscriber(s *Subscriber) error {
	t.Lock()
	defer t.Unlock()

	select {
	case <-t.done:
		return ErrClosedTransport
	default:
	}

	t.subscribers[s] = struct{}{}

	return nil
}

// Close closes the Transport.
func (t *LocalTransport) Close() error {
	select {
	case <-t.done:
		return nil
	default:
	}

	t.RLock()
	defer t.RUnlock()
	for subscriber := range t.subscribers {
		subscriber.Disconnect()
		delete(t.subscribers, subscriber)
	}
	close(t.done)

	return nil
}
