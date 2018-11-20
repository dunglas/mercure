package hub

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncodeFull(t *testing.T) {
	e := &Event{"several\nlines\rwith\r\neol", "custom-id", "type", 5}

	assert.Equal(t, "event: type\nretry: 5\nid: custom-id\ndata: several\ndata: lines\ndata: with\ndata: eol\n\n", e.String())
}

func TestEncodeNoType(t *testing.T) {
	e := &Event{"data", "custom-id", "", 5}

	assert.Equal(t, "retry: 5\nid: custom-id\ndata: data\n\n", e.String())
}

func TestEncodeNoRetry(t *testing.T) {
	e := &Event{"data", "custom-id", "", 0}

	assert.Equal(t, "id: custom-id\ndata: data\n\n", e.String())
}
