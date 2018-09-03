package hub

import (
	"testing"

	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
)

func TestGenerateID(t *testing.T) {
	u := NewEvent("", "", "", 0)

	_, err := uuid.FromString(u.ID)
	assert.Nil(t, err)
}

func TestEncodeFull(t *testing.T) {
	u := NewEvent("several\nlines\rwith\r\neol", "custom-id", "type", 5)

	assert.Equal(t, "event: type\nretry: 5\nid: custom-id\ndata: several\ndata: lines\ndata: with\ndata: eol\n\n", u.String())
}

func TestEncodeNoType(t *testing.T) {
	u := NewEvent("data", "custom-id", "", 5)

	assert.Equal(t, "retry: 5\nid: custom-id\ndata: data\n\n", u.String())
}

func TestEncodeNoRetry(t *testing.T) {
	u := NewEvent("data", "custom-id", "", 0)

	assert.Equal(t, "id: custom-id\ndata: data\n\n", u.String())
}
