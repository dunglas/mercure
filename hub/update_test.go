package hub

import (
	"strings"
	"testing"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
)

func TestAssingUUID(t *testing.T) {
	u := &Update{
		Topics:  []string{"foo"},
		Private: true,
		Event:   Event{Retry: 3},
	}
	AssignUUID(u)

	assert.Equal(t, []string{"foo"}, u.Topics)
	assert.True(t, u.Private)
	assert.Equal(t, uint64(3), u.Retry)
	assert.True(t, strings.HasPrefix(u.ID, "urn:uuid:"))

	_, err := uuid.FromString(strings.TrimPrefix(u.ID, "urn:uuid:"))
	assert.Nil(t, err)
}
