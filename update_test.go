package mercure

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssignUUID(t *testing.T) {
	t.Parallel()

	u := &Update{
		Topics:  []string{"foo"},
		Private: true,
		Event:   Event{Retry: 3},
	}
	u.AssignUUID()

	assert.Equal(t, []string{"foo"}, u.Topics)
	assert.True(t, u.Private)
	assert.Equal(t, uint64(3), u.Retry)
	assert.True(t, strings.HasPrefix(u.ID, "urn:uuid:"))

	_, err := uuid.FromString(strings.TrimPrefix(u.ID, "urn:uuid:"))
	require.NoError(t, err)
}

// TestUpdateJSON guards the wire format used by bolt/redis history: the
// canonical topic and its alternates round-trip as a single "Topics" array,
// matching the 0.x shape exactly.
func TestUpdateJSON(t *testing.T) {
	t.Parallel()

	legacy := `{"Data":"d","ID":"i","Type":"t","Retry":3,"Topics":["https://example.com/a","https://example.com/b"],"Private":true,"Debug":false}`

	var u *Update

	require.NoError(t, json.Unmarshal([]byte(legacy), &u))
	assert.Equal(t, []string{"https://example.com/a", "https://example.com/b"}, u.Topics)
	assert.Equal(t, "d", u.Data)
	assert.Equal(t, "i", u.ID)
	assert.Equal(t, "t", u.Type)
	assert.Equal(t, uint64(3), u.Retry)
	assert.True(t, u.Private)

	out, err := json.Marshal(u)
	require.NoError(t, err)
	assert.JSONEq(t, legacy, string(out))
}

// Binary payloads must survive JSON persistence byte-exactly: encoding/json
// replaces invalid UTF-8 with U+FFFD, so MarshalJSON base64-encodes Data
// when Binary is set.
func TestUpdateBinaryJSONRoundTrip(t *testing.T) {
	t.Parallel()

	u := &Update{
		Topics: []string{"https://example.com/books/1"},
		Binary: true,
		Event:  Event{Data: "\xff\x00\xfePNG", ID: "i"},
	}

	serialized, err := json.Marshal(u)
	require.NoError(t, err)
	assert.Contains(t, string(serialized), `"Data":"/wD+UE5H"`)
	assert.Contains(t, string(serialized), `"Binary":true`)

	var decoded Update

	require.NoError(t, json.Unmarshal(serialized, &decoded))
	assert.Equal(t, *u, decoded)
}

func TestUpdateValidateBinaryData(t *testing.T) {
	t.Parallel()

	u := &Update{Topics: []string{"https://example.com/books/1"}, Event: Event{Data: "\xff\x00"}}
	require.ErrorIs(t, u.Validate(), ErrInvalidData)

	u.Binary = true
	require.NoError(t, u.Validate())
}

func TestLogUpdate(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	u := &Update{
		Topics:  []string{"https://example.com/foo"},
		Private: true,
		Debug:   true,
		Event:   Event{ID: "a", Retry: 3, Data: "bar", Type: "baz"},
	}

	logger.Info("test", slog.Any("update", u))

	log := buf.String()
	assert.Contains(t, log, `"id":"a"`)
	assert.Contains(t, log, `"type":"baz"`)
	assert.Contains(t, log, `"retry":3`)
	assert.Contains(t, log, `"topics":["https://example.com/foo"]`)
	assert.Contains(t, log, `"private":true`)
	assert.Contains(t, log, `"data":"bar"`)
}
