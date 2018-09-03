package hub

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewUpdate(t *testing.T) {
	u := NewUpdate(
		[]string{"http://example.com/canonical", "http://example.com/alternate"},
		map[string]struct{}{"user-id": struct{}{}, "group-id": struct{}{}},
		"data",
		"id",
		"type",
		5,
	)

	assert.IsType(t, Update{}, u)
	assert.Equal(t, []string{"http://example.com/canonical", "http://example.com/alternate"}, u.Topics)
	assert.Equal(t, "data", u.Event.Data)
	assert.Equal(t, "id", u.Event.ID)
	assert.Equal(t, "type", u.Event.Type)
	assert.Equal(t, uint64(5), u.Event.Retry)
}
