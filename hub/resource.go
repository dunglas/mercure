package hub

import (
	"fmt"
	"strings"

	uuid "github.com/satori/go.uuid"
)

// Resource contains a server-sent event
type Resource struct {
	// The unique id corresponding to this version of this resource, will be used as the SSE id
	RevID string

	// The Internationalized Resource Identifier (RFC3987) of the resource (will most likely be an URI)
	IRI string

	// Data, encoded in the sever-sent event format: every line starts with the string "data: "
	// https://www.w3.org/TR/eventsource/#dispatchMessage
	Data string

	// Target audience
	Targets map[string]struct{}
}

// NewResource creates a new resource and encodes the data property
func NewResource(revID, iri, data string, targets map[string]struct{}) Resource {
	if revID == "" {
		revID = uuid.Must(uuid.NewV4()).String()
	}

	return Resource{revID, iri, fmt.Sprintf("data: %s\n\n", strings.Replace(data, "\n", "\ndata: ", -1)), targets}
}
