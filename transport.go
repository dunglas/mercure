package mercure

import (
	"context"
	"errors"
	"fmt"
)

// EarliestLastEventID is the reserved value representing the earliest available event id.
const EarliestLastEventID = "earliest"

// Transport provides methods to dispatch and persist updates.
type Transport interface {
	// Dispatch dispatches an update to all subscribers.
	Dispatch(ctx context.Context, u *Update) error

	// AddSubscriber adds a new subscriber to the transport.
	AddSubscriber(ctx context.Context, s *LocalSubscriber) error

	// RemoveSubscriber removes a subscriber from the transport.
	RemoveSubscriber(ctx context.Context, s *LocalSubscriber) error

	// Close closes the Transport.
	Close(ctx context.Context) error
}

// TransportSubscribers provides a method to retrieve the list of active subscribers.
type TransportSubscribers interface {
	// GetSubscribers gets the last event ID and the list of active subscribers at this time.
	GetSubscribers(ctx context.Context) (string, []*Subscriber, error)
}

// TransportTopicSelectorStore provides a method to pass the TopicSelectorStore to the transport.
type TransportTopicSelectorStore interface {
	SetTopicSelectorStore(store *TopicSelectorStore)
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
	sl.Walk(0, func(s *LocalSubscriber) bool {
		subscribers = append(subscribers, &s.Subscriber)

		return true
	})

	return subscribers
}
