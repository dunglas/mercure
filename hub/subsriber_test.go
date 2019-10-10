package hub

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSubscribed(t *testing.T) {
	s := NewSubscriber(false, nil, []string{"foo", "bar"}, []string{"foo", "bar"}, nil, "lid")
	assert.False(t, s.IsSubscribed(&Update{Topics: []string{"baz", "bat"}}))
	assert.True(t, s.IsSubscribed(&Update{Topics: []string{"baz", "bar"}}))
}
