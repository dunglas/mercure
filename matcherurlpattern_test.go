package mercure

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestURLPatternMatcher(t *testing.T) {
	t.Parallel()

	m := urlPatternMatcherType{}

	// Named group matching
	assert.True(t, m.Match([]string{"https://example.com/books/123"}, "https://example.com/books/:id"))
	assert.True(t, m.Match([]string{"https://example.com/books/abc"}, "https://example.com/books/:id"))

	// No match — different path structure
	assert.False(t, m.Match([]string{"https://example.com/authors/123"}, "https://example.com/books/:id"))

	// Wildcard path
	assert.True(t, m.Match([]string{"https://example.com/anything"}, "https://example.com/*"))
	assert.True(t, m.Match([]string{"https://example.com/a/b/c"}, "https://example.com/*"))

	// Exact URL match
	assert.True(t, m.Match([]string{"https://example.com/foo"}, "https://example.com/foo"))
	assert.False(t, m.Match([]string{"https://example.com/bar"}, "https://example.com/foo"))

	// Multiple named groups
	assert.True(t, m.Match([]string{"https://example.com/users/42/posts/99"}, "https://example.com/users/:uid/posts/:pid"))
	assert.False(t, m.Match([]string{"https://example.com/users/42"}, "https://example.com/users/:uid/posts/:pid"))

	// Multiple topics — at least one matches
	assert.True(t, m.Match([]string{"https://example.com/authors/123", "https://example.com/books/123"}, "https://example.com/books/:id"))
}

func TestURLPatternMatcherValidate(t *testing.T) {
	t.Parallel()

	m := urlPatternMatcherType{}

	assert.NoError(t, m.Validate("https://example.com/books/:id"))
	assert.NoError(t, m.Validate("*://example.com/books/:id"))

	// Relative patterns must be rejected: topics are absolute IRIs and the
	// protocol exposes no base URL to resolve against.
	err := m.Validate("/books/:id")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "relative URL")

	err = m.Validate("books/:id")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "relative URL")
}
