package mercure

import (
	"fmt"

	"github.com/gofrs/uuid"
	"go.uber.org/zap/zapcore"
)

// Update represents an update to send to subscribers.
type Update struct {
	// The topics' Internationalized Resource Identifier (RFC3987) (will most likely be URLs).
	// The first one is the canonical IRI, while next ones are alternate IRIs.
	Topics []string

	// Private updates can only be dispatched to subscribers authorized to receive them.
	Private bool

	// To print debug informations
	Debug bool

	// The Server-Sent Event to send.
	Event
}

func (u *Update) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("id", u.ID)
	enc.AddString("type", u.Type)
	enc.AddUint64("retry", u.Retry)
	if err := enc.AddArray("topics", stringArray(u.Topics)); err != nil {
		return fmt.Errorf("log error: %w", err)
	}
	enc.AddBool("private", u.Private)

	if u.Debug {
		enc.AddString("data", u.Data)
	}

	return nil
}

type serializedUpdate struct {
	*Update
	event string
}

// AssignUUID generates a new UUID an assign it to the given update if no ID is already set.
func AssignUUID(u *Update) {
	if u.ID == "" {
		u.ID = "urn:uuid:" + uuid.Must(uuid.NewV4()).String()
	}
}

func newSerializedUpdate(u *Update) *serializedUpdate {
	return &serializedUpdate{u, u.String()}
}
