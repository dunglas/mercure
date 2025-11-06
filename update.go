package mercure

import (
	"log/slog"

	"github.com/gofrs/uuid/v5"
)

// Update represents an update to send to subscribers.
type Update struct {
	// The Server-Sent Event to send.
	Event

	// The topics' Internationalized Resource Identifier (RFC3987) (will most likely be URLs).
	// The first one is the canonical IRI, while next ones are alternate IRIs.
	Topics []string

	// Private updates can only be dispatched to subscribers authorized to receive them.
	Private bool

	// To print debug information
	Debug bool
}

func (u *Update) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("id", u.ID),
		slog.String("type", u.Type),
		slog.Uint64("retry", u.Retry),
		slog.Any("topics", u.Topics),
		slog.Bool("private", u.Private),
	}

	if u.Debug {
		attrs = append(attrs, slog.String("data", u.Data))
	}

	return slog.GroupValue(attrs...)
}

type serializedUpdate struct {
	*Update

	event string
}

// AssignUUID generates a new UUID an assign it to the given update if no ID is already set.
func (u *Update) AssignUUID() {
	if u.ID == "" {
		u.ID = "urn:uuid:" + uuid.Must(uuid.NewV7()).String()
	}
}

func newSerializedUpdate(u *Update) *serializedUpdate {
	return &serializedUpdate{u, u.String()}
}
