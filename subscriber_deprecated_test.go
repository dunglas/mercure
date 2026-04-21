//go:build deprecated_topic

package mercure

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchTopic(t *testing.T) {
	t.Parallel()

	s := NewLocalSubscriber("", slog.Default(), &TopicSelectorStore{})
	s.setMatchers(stringsToDeprecatedMatchers([]string{"https://example.com/no-match", "https://example.com/books/{id}"}), stringsToDeprecatedMatchers([]string{"https://example.com/users/foo/{?topic}"}))

	assert.False(t, s.Match(&Update{Topics: []string{"https://example.com/not-subscribed"}}))
	assert.False(t, s.Match(&Update{Topics: []string{"https://example.com/not-subscribed"}, Private: true}))
	assert.False(t, s.Match(&Update{Topics: []string{"https://example.com/no-match"}, Private: true}))
	assert.False(t, s.Match(&Update{Topics: []string{"https://example.com/books/1"}, Private: true}))
	assert.False(t, s.Match(&Update{Topics: []string{"https://example.com/books/1", "https://example.com/users/bar/?topic=https%3A%2F%2Fexample.com%2Fbooks%2F1"}, Private: true}))

	assert.True(t, s.Match(&Update{Topics: []string{"https://example.com/books/1"}}))
	assert.True(t, s.Match(&Update{Topics: []string{"https://example.com/books/1", "https://example.com/users/foo/?topic=https%3A%2F%2Fexample.com%2Fbooks%2F1"}, Private: true}))
}
