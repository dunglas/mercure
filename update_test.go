package mercure

import (
	"bytes"
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
