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

	// The canonical topic's Internationalized Resource Identifier (RFC3987)
	// (will most likely be a URL).
	Topic string

	// Alternate topic IRIs. The update is dispatched to subscribers matching
	// either the canonical topic or any alternate; a private update's
	// audience is the union of the audiences of all its topics.
	Topics []string

	// Private updates can only be dispatched to subscribers authorized to receive them.
	Private bool

	// To print debug information
	Debug bool
}

// updateJSON is the wire shape: the canonical topic and its alternates,
// serialized together as a "Topics" array (canonical first).
type updateJSON struct {
	Event

	Topics  []string
	Private bool
	Debug   bool
}

func (u *Update) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(updateJSON{Event: u.Event, Topics: u.topics(), Private: u.Private, Debug: u.Debug})
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

// topics returns the canonical topic followed by its alternates. Updates
// built by legacy code that only sets Topics keep working: the first element
// acts as the canonical topic.
func (u *Update) topics() []string {
	if u.Topic == "" && len(u.Topics) > 0 {
		return u.Topics
	}

	if len(u.Topics) == 0 {
		return []string{u.Topic}
	}

	return append([]string{u.Topic}, u.Topics...)
}

// setTopics assigns the canonical topic and its alternates from a topic list
// (canonical first), as carried by the "topic" publish field or the wire
// shape.
func (u *Update) setTopics(topics []string) {
	u.Topic, u.Topics = "", nil

	if len(topics) == 0 {
		return
	}

	u.Topic = topics[0]
	if len(topics) > 1 {
		u.Topics = topics[1:]
	}
}
