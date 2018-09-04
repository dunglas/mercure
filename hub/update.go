package hub

// Update represents an update to send to subscribers
type Update struct {
	// The topics' Internationalized Resource Identifier (RFC3987) (will most likely be URLs)
	// The first one is the canonical IRI, while next ones are alternate IRIs
	Topics []string

	// The target audience
	Targets map[string]struct{}

	// The Server-Sent Event to send
	Event
}

// NewUpdate creates a new update to dispatch
func NewUpdate(topics []string, targets map[string]struct{}, eventData, eventID, eventType string, eventRetry uint64) *Update {
	return &Update{topics, targets, NewEvent(eventData, eventID, eventType, eventRetry)}
}
