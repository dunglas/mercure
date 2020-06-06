package hub

import "github.com/gofrs/uuid"

// Update represents an update to send to subscribers.
type Update struct {
	// The topics' Internationalized Resource Identifier (RFC3987) (will most likely be URLs).
	// The first one is the canonical IRI, while next ones are alternate IRIs.
	Topics []string

	// Private updates can only be dispatched to subscribers authorized to receive them.
	Private bool

	// The Server-Sent Event to send.
	Event
}

type serializedUpdate struct {
	*Update
	event string
}

// AssignUUID generates a new UUID an assign it to the given update if no ID is already set.
func AssignUUID(u *Update) {
	if u.ID == "" {
		u.ID = "urn:uuid:" + uuid.Must(uuid.NewV4()).String()
	}
}

func newSerializedUpdate(u *Update) *serializedUpdate {
	return &serializedUpdate{u, u.String()}
}
