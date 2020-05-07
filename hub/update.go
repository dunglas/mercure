package hub

import "github.com/gofrs/uuid"

// Update represents an update to send to subscribers.
type Update struct {
	// The target audience.
	Targets map[string]struct{}

	// The topics' Internationalized Resource Identifier (RFC3987) (will most likely be URLs).
	// The first one is the canonical IRI, while next ones are alternate IRIs.
	Topics []string

	// The Server-Sent Event to send.
	Event
}

type serializedUpdate struct {
	*Update
	event string
}

func newUpdate(event Event, topics []string, targets map[string]struct{}) *Update {
	u := &Update{
		Event:   event,
		Topics:  topics,
		Targets: targets,
	}
	if u.ID == "" {
		u.ID = uuid.Must(uuid.NewV4()).String()
	}

	return u
}

func newSerializedUpdate(u *Update) *serializedUpdate {
	return &serializedUpdate{u, u.String()}
}
