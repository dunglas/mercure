//go:build deprecated_topic

package mercure

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stringsToDeprecatedMatchers(patterns []string) []topicMatcher {
	out := make([]topicMatcher, len(patterns))
	for i, p := range patterns {
		out[i] = topicMatcher{Type: deprecatedMatcherTypeName, Pattern: p}
	}

	return out
}

// TestMatchAlternateTopics covers the v8 rules: URI Template selectors and
// alternate topics — an update is receivable when any of its topics
// (canonical or alternate) matches a subscriber selector.
func TestMatchAlternateTopics(t *testing.T) {
	t.Parallel()

	tss, err := NewTopicSelectorStore(0)
	require.NoError(t, err)

	s := NewLocalSubscriber("", slog.Default(), tss)
	s.setMatchers(
		stringsToDeprecatedMatchers([]string{"https://example.com/no-match", "https://example.com/books/{id}"}),
		stringsToDeprecatedMatchers([]string{"https://example.com/users/foo/{?topic}"}),
	)

	// URI Template selectors keep working for v8 subscribers.
	assert.True(t, s.Match(&Update{Topic: "https://example.com/books/1"}))
	assert.False(t, s.Match(&Update{Topic: "https://example.com/books/1", Private: true}))

	// The alternate topic does not match the private selector.
	assert.False(t, s.Match(testUpdate(&Update{Private: true},
		"https://example.com/books/1", "https://example.com/users/bar/?topic=https%3A%2F%2Fexample.com%2Fbooks%2F1")))

	// The alternate topic matches the private selector.
	assert.True(t, s.Match(testUpdate(&Update{Private: true},
		"https://example.com/books/1", "https://example.com/users/foo/?topic=https%3A%2F%2Fexample.com%2Fbooks%2F1")))
}
