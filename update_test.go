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
		Topic:   "foo",
		Private: true,
		Event:   Event{Retry: 3},
	}
	u.AssignUUID()

	assert.Equal(t, "foo", u.Topic)
	assert.True(t, u.Private)
	assert.Equal(t, uint64(3), u.Retry)
	assert.True(t, strings.HasPrefix(u.ID, "urn:uuid:"))

	_, err := uuid.FromString(strings.TrimPrefix(u.ID, "urn:uuid:"))
	require.NoError(t, err)
}

// TestUpdateJSONLegacyShape guards the wire format used by bolt: records
// written by 0.x hubs carry a "Topics" array and must stay readable, and
// records written by this version must keep the same shape.
func TestUpdateJSONLegacyShape(t *testing.T) {
	t.Parallel()

	legacy := `{"Data":"d","ID":"i","Type":"t","Retry":3,"Topics":["https://example.com/a","https://example.com/b"],"Private":true,"Debug":false}`

	var u *Update

	require.NoError(t, json.Unmarshal([]byte(legacy), &u))
	assert.Equal(t, "https://example.com/a", u.Topic)
	assert.Equal(t, "d", u.Data)
	assert.Equal(t, "i", u.ID)
	assert.Equal(t, "t", u.Type)
	assert.Equal(t, uint64(3), u.Retry)
	assert.True(t, u.Private)

	out, err := json.Marshal(u)
	require.NoError(t, err)
	assert.Contains(t, string(out), `"Topics":["https://example.com/a"`)
}

func TestLogUpdate(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	u := &Update{
		Topic:   "https://example.com/foo",
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
