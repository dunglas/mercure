package mercure

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/gofrs/uuid/v5"
	"go.opentelemetry.io/otel/attribute"
)

// Update represents an update to send to subscribers.
type Update struct {
	// The Server-Sent Event to send.
	Event
	// v8 alternate topics; only present in builds with the deprecated_topic tag.
	deprecatedTopics //nolint:unused // empty struct without the deprecated_topic tag

	// The topic's Internationalized Resource Identifier (RFC3987) (will most likely be a URL).
	Topic string

	// Private updates can only be dispatched to subscribers authorized to receive them.
	Private bool

	// To print debug information
	Debug bool
}

// updateJSON preserves the historic wire shape (a "Topics" array holding the
// canonical topic first) so bolt databases written by 0.x hubs stay readable
// in both build modes.
type updateJSON struct {
	Event

	Topics  []string
	Private bool
	Debug   bool
}

func (u *Update) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(updateJSON{u.Event, u.topics(), u.Private, u.Debug})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal update: %w", err)
	}

	return b, nil
}

func (u *Update) UnmarshalJSON(data []byte) error {
	var j updateJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return err //nolint:wrapcheck
	}

	*u = Update{Event: j.Event, Private: j.Private, Debug: j.Debug}
	u.setTopics(j.Topics)

	return nil
}

func (u *Update) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("id", u.ID),
		slog.String("type", u.Type),
		slog.Uint64("retry", u.Retry),
		slog.Any("topics", u.topics()),
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

// SpanAttributes returns the OpenTelemetry attributes describing this update.
func (u *Update) SpanAttributes() []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, 3)
	if u.ID != "" {
		attrs = append(attrs, attribute.String("mercure.update.id", u.ID))
	}

	return append(attrs,
		attribute.StringSlice("mercure.topics", u.topics()),
		attribute.Bool("mercure.private", u.Private),
	)
}

func newSerializedUpdate(u *Update) *serializedUpdate {
	return &serializedUpdate{u, u.String()}
}
