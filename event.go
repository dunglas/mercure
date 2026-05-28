package mercure

import (
	"fmt"
	"strings"
)

//nolint:gochecknoglobals
var dataReplacer = strings.NewReplacer("\r\n", "\ndata: ", "\r", "\ndata: ", "\n", "\ndata: ")

// sseFieldForbiddenChars are characters that, if present in an SSE
// header field such as id: or event:, would let a publisher inject
// arbitrary SSE fields into the stream seen by subscribers.
const sseFieldForbiddenChars = "\x00\r\n"

// containsSSEFieldForbiddenChar reports whether s contains a character
// that must not appear in an SSE header field (id, event, retry).
func containsSSEFieldForbiddenChar(s string) bool {
	return strings.ContainsAny(s, sseFieldForbiddenChars)
}

// Event is the actual Server Sent Event that will be dispatched.
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

// String serializes the event in a "text/event-stream" representation.
func (e *Event) String() string {
	var b strings.Builder

	if e.Type != "" {
		_, _ = fmt.Fprintf(&b, "event: %s\n", e.Type)
	}

	if e.Retry != 0 {
		_, _ = fmt.Fprintf(&b, "retry: %d\n", e.Retry)
	}

	_, _ = fmt.Fprintf(&b, "id: %s\ndata: %s\n\n", e.ID, dataReplacer.Replace(e.Data))

	return b.String()
}
