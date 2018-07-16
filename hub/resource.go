package hub

import (
	"fmt"
	"strings"
)

// Resource contains a server-sent event
type Resource struct {
	// The Internationalized Resource Identifier (RFC3987) of the resource (will most likely be an URI), prefixed by "id: "
	IRI string

	// Data, encoded in the sever-sent event format: every line starts with the string "data: "
	// https://www.w3.org/TR/eventsource/#dispatchMessage
	Data string

	// Target audience
	Targets map[string]bool
}

// NewResource creates a new resource and encodes the data property
func NewResource(iri string, data string, targets map[string]bool) Resource {
	return Resource{iri, fmt.Sprintf("data: %s\n\n", strings.Replace(data, "\n", "\ndata: ", -1)), targets}
}
