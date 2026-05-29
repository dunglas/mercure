package mercure

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExactMatcher(t *testing.T) {
	t.Parallel()

	m := exactMatcherType{}

	assert.True(t, m.Match([]string{"foo"}, "foo"))
	assert.True(t, m.Match([]string{"https://example.com/foo"}, "https://example.com/foo"))
	assert.True(t, m.Match([]string{""}, ""))

	assert.False(t, m.Match([]string{"foo"}, "bar"))
	assert.False(t, m.Match([]string{"foo"}, "Foo"))
	assert.False(t, m.Match([]string{"foo"}, "foo "))
	assert.False(t, m.Match([]string{"https://example.com/foo"}, "https://example.com/bar"))

	// Multiple topics — at least one matches
	assert.True(t, m.Match([]string{"bar", "foo"}, "foo"))
	assert.False(t, m.Match([]string{"bar", "baz"}, "foo"))

	// Wildcard is NOT handled by ExactMatcher — it's handled at the TopicSelectorStore level
	assert.False(t, m.Match([]string{"foo"}, "*"))
}
