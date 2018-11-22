package hub

// Update represents an update to send to subscribers
type Update struct {
	// The target audience
	Targets map[string]struct{}

	// The topics' Internationalized Resource Identifier (RFC3987) (will most likely be URLs)
	// The first one is the canonical IRI, while next ones are alternate IRIs
	Topics []string

	// The Server-Sent Event to send
	Event
}

type serializedUpdate struct {
	*Update
	event string
}

func newSerializedUpdate(u *Update) *serializedUpdate {
	return &serializedUpdate{u, u.String()}
}
