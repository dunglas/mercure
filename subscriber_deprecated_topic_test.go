//go:build deprecated_topic

package mercure

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMatchAlternateTopics covers the v8 alternate-topic rule: an update is
// receivable when any of its topics (canonical or alternate) matches a
// subscriber selector.
func TestMatchAlternateTopics(t *testing.T) {
	t.Parallel()

	s := NewLocalSubscriber("", slog.Default(), &TopicSelectorStore{})
	s.SetTopics([]string{"https://example.com/no-match", "https://example.com/books/{id}"}, []string{"https://example.com/users/foo/{?topic}"})

	// The alternate topic does not match the private selector.
	assert.False(t, s.Match(testUpdate(&Update{Private: true},
		"https://example.com/books/1", "https://example.com/users/bar/?topic=https%3A%2F%2Fexample.com%2Fbooks%2F1")))

	// The alternate topic matches the private selector.
	assert.True(t, s.Match(testUpdate(&Update{Private: true},
		"https://example.com/books/1", "https://example.com/users/foo/?topic=https%3A%2F%2Fexample.com%2Fbooks%2F1")))
}
