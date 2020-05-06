package hub

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSubscribed(t *testing.T) {
	s := newSubscriber()
	s.topics = []string{"foo", "bar"}
	s.rawTopics = s.topics

	assert.Len(t, s.matchCache, 0)
	assert.False(t, s.IsSubscribed(&Update{Topics: []string{"baz", "bat"}}))
	assert.True(t, s.IsSubscribed(&Update{Topics: []string{"baz", "bar"}}))
	assert.Len(t, s.matchCache, 3)

	// assert cache is used
	assert.True(t, s.IsSubscribed(&Update{Topics: []string{"bar", "qux"}}))
	assert.Len(t, s.matchCache, 3)
}
