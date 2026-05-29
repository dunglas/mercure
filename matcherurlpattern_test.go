package mercure

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestURLPatternMatcher(t *testing.T) {
	t.Parallel()

	m := NewURLPatternMatcher("")

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

// TestURLPatternMatcherRelative covers the spec case where both pattern and
// topic are relative — the shape the hub uses when it dispatches subscription
// events on `/.well-known/mercure/subscriptions/{matchType}/{match}/{subscriber}`
// topics. Relative ↔ relative must match; relative ↔ absolute must not.
func TestURLPatternMatcherRelative(t *testing.T) {
	t.Parallel()

	m := NewURLPatternMatcher("")

	// Relative pattern matches relative topic resolved against the same base.
	assert.True(t, m.Match(
		[]string{"/.well-known/mercure/subscriptions/Exact/foo/bar"},
		"/.well-known/mercure/subscriptions/Exact/:topic/:subscriber",
	))
	assert.True(t, m.Match([]string{"/books/123"}, "/books/:id"))
	assert.False(t, m.Match([]string{"/authors/123"}, "/books/:id"))

	// A relative pattern is anchored at the hub origin: an absolute topic on
	// a different origin must not match.
	assert.False(t, m.Match([]string{"https://example.com/books/123"}, "/books/:id"))

	// Symmetric: an absolute pattern does not match a relative topic that
	// resolves to a different origin.
	assert.False(t, m.Match([]string{"/books/123"}, "https://example.com/books/:id"))
}

// TestURLPatternMatcherConfiguredBase exercises the case the synthetic
// fallback cannot handle: a relative pattern matches an absolute topic on
// the hub URL (and vice-versa) when the matcher is built with the real
// hub URL as base.
func TestURLPatternMatcherConfiguredBase(t *testing.T) {
	t.Parallel()

	m := NewURLPatternMatcher("https://hub.example.com")

	// Relative pattern + absolute topic on the hub URL → match.
	assert.True(t, m.Match([]string{"https://hub.example.com/books/123"}, "/books/:id"))

	// Absolute pattern on the hub URL + relative topic → match.
	assert.True(t, m.Match([]string{"/books/123"}, "https://hub.example.com/books/:id"))

	// Different origin still does not match.
	assert.False(t, m.Match([]string{"https://other.example.com/books/123"}, "/books/:id"))
}

func TestURLPatternMatcherValidate(t *testing.T) {
	t.Parallel()

	v, ok := NewURLPatternMatcher("").(PatternValidator)
	require.True(t, ok)

	// Both absolute and relative patterns are accepted (relative ones are
	// anchored at the hub URL per the spec).
	assert.NoError(t, v.Validate("https://example.com/books/:id"))
	assert.NoError(t, v.Validate("*://example.com/books/:id"))
	assert.NoError(t, v.Validate("/books/:id"))
	assert.NoError(t, v.Validate("/.well-known/mercure/subscriptions/:matchType/:match/:subscriber"))

	// Genuinely malformed patterns still fail.
	assert.Error(t, v.Validate("{unclosed"))
}
