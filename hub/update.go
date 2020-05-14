package hub

import "github.com/gofrs/uuid"

// Update represents an update to send to subscribers.
type Update struct {
	// The topics' Internationalized Resource Identifier (RFC3987) (will most likely be URLs).
	// The first one is the canonical IRI, while next ones are alternate IRIs.
	Topics []string

	// Private updates can only be dispatched to subscribers authorized to receive them.
	Private bool

	// PreviousID contains the ID of the previous update
	// This value must be sent only if the request Last-Event-ID cannot be found, and only on the first update available
	PreviousID string

	// The Server-Sent Event to send.
	Event
}

type serializedUpdate struct {
	*Update
	event string
}

func newUpdate(topics []string, private bool, event Event) *Update {
	u := &Update{
		Topics:  topics,
		Private: private,
		Event:   event,
	}
	if u.ID == "" {
		u.ID = "urn:uuid:" + uuid.Must(uuid.NewV4()).String()
	}

	return u
}

func newSerializedUpdate(u *Update) *serializedUpdate {
	return &serializedUpdate{u, u.String()}
}
