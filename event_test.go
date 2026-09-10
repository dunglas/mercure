package mercure

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncodeFull(t *testing.T) {
	t.Parallel()

	e := &Event{"several\nlines\rwith\r\neol", "custom-id", "type", 5}

	assert.Equal(t, "event: type\nretry: 5\nid: custom-id\ndata: several\ndata: lines\ndata: with\ndata: eol\n\n", e.String())
}

func TestEncodeNoType(t *testing.T) {
	t.Parallel()

	e := &Event{"data", "custom-id", "", 5}

	assert.Equal(t, "retry: 5\nid: custom-id\ndata: data\n\n", e.String())
}

func TestEncodeNoRetry(t *testing.T) {
	t.Parallel()

	e := &Event{"data", "custom-id", "", 0}

	assert.Equal(t, "id: custom-id\ndata: data\n\n", e.String())
}

func TestEncodeTopics(t *testing.T) {
	t.Parallel()

	e := &Event{"data", "custom-id", "type", 0}

	assert.Equal(
		t,
		"event: type\ntopic: https://example.com/books/1\ntopic: https://example.com/alt/1\nid: custom-id\ndata: data\n\n",
		e.serialize([]string{"https://example.com/books/1", "https://example.com/alt/1"}),
	)
}
