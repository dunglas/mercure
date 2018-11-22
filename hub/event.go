package hub

import (
	"fmt"
	"strings"
)

// Event is the actual Server Sent Event that will be dispatched
type Event struct {
	// The updates' data, encoded in the sever-sent event format: every line starts with the string "data: "
	// https://www.w3.org/TR/eventsource/#dispatchMessage
	Data string

	// The globally unique identifier corresponding to update
	ID string

	// The event type, will be attached to the "event" field
	Type string

	// The reconnection time
	Retry uint64
}

// String serializes the event in a "text/event-stream" representation
func (e *Event) String() string {
	var b strings.Builder

	if e.Type != "" {
		fmt.Fprintf(&b, "event: %s\n", e.Type)
	}
	if e.Retry != 0 {
		fmt.Fprintf(&b, "retry: %d\n", e.Retry)
	}

	r := strings.NewReplacer("\r\n", "\ndata: ", "\r", "\ndata: ", "\n", "\ndata: ")
	fmt.Fprintf(&b, "id: %s\ndata: %s\n\n", e.ID, r.Replace(e.Data))

	return b.String()
}
