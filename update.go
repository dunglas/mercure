package mercure

import (
	"encoding/base64"
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

	// The topics' Internationalized Resource Identifier (RFC3987) (will most
	// likely be URLs). The first one is the canonical topic; any others are
	// alternate topics. The update is dispatched to subscribers matching
	// either the canonical topic or any alternate; a private update's
	// audience is the union of the audiences of all its topics.
	Topics []string

	// Private updates can only be dispatched to subscribers authorized to receive them.
	Private bool

	// The media type of Data, as declared by the publisher. Conveyed to
	// subscribers when the negotiated response encoding can carry per-event
	// metadata; text/event-stream defines no field for it. omitempty keeps
	// history entries persisted before this field existed decodable.
	ContentType string `json:",omitempty"`

	// Binary marks updates whose Data is an arbitrary byte payload
	// (published as multipart/form-data): Data is base64-encoded when
	// serialized to text formats (SSE, JSON) and delivered verbatim
	// otherwise. omitempty keeps older history entries decodable.
	Binary bool `json:",omitempty"`

	// To print debug information
	Debug bool
}

// updateJSON drops Update's methods so encoding/json does not recurse into
// MarshalJSON; the wire shape stays byte-identical for non-binary updates.
type updateJSON Update

// MarshalJSON base64-encodes binary Data: encoding/json silently replaces
// invalid UTF-8 in strings with U+FFFD, which would corrupt persisted
// binary payloads.
func (u *Update) MarshalJSON() ([]byte, error) {
	c := updateJSON(*u)
	if u.Binary {
		c.Data = base64.StdEncoding.EncodeToString([]byte(u.Data))
	}

	b, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal update: %w", err)
	}

	return b, nil
}

func (u *Update) UnmarshalJSON(data []byte) error {
	var c updateJSON
	if err := json.Unmarshal(data, &c); err != nil {
		return fmt.Errorf("unable to unmarshal update: %w", err)
	}

	if c.Binary {
		d, err := base64.StdEncoding.DecodeString(c.Data)
		if err != nil {
			return fmt.Errorf("unable to decode binary update data: %w", err)
		}

		c.Data = string(d)
	}

	*u = Update(c)

	return nil
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
		attribute.StringSlice("mercure.topics", u.Topics),
		attribute.Bool("mercure.private", u.Private),
	)
}
