package hub

import (
	"strings"
	"testing"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
)

func TestNewUpdate(t *testing.T) {
	u := newUpdate([]string{"foo"}, true, Event{Retry: 3})

	assert.Equal(t, []string{"foo"}, u.Topics)
	assert.True(t, u.Private)
	assert.Equal(t, uint64(3), u.Retry)

	assert.True(t, strings.HasPrefix(u.ID, "urn:uuid:"))

	_, err := uuid.FromString(strings.TrimPrefix(u.ID, "urn:uuid:"))
	assert.Nil(t, err)
}
