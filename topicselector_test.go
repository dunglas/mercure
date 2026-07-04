package mercure

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func exactMatcher(pattern string) TopicMatcher {
	return TopicMatcher{Type: MatcherTypeExact, Pattern: pattern}
}

func urlPatternMatcher(pattern string) TopicMatcher {
	return TopicMatcher{Type: MatcherTypeURLPattern, Pattern: pattern}
}

func TestMatchExact(t *testing.T) {
	t.Parallel()

	tss, err := NewTopicSelectorStore(DefaultTopicSelectorStoreCacheSize)
	require.NoError(t, err)

	assert.True(t, tss.matchMatcher([]string{"foo"}, exactMatcher("foo")))
	assert.False(t, tss.matchMatcher([]string{"foo"}, exactMatcher("bar")))
	assert.False(t, tss.matchMatcher([]string{"foo"}, exactMatcher("FOO")))
	assert.True(t, tss.matchMatcher([]string{"bar", "foo"}, exactMatcher("foo")))

	// The reserved wildcard always matches.
	assert.True(t, tss.matchMatcher([]string{"https://example.com/foo/bar"}, exactMatcher("*")))

	// Exact patterns are never interpreted as templates.
	assert.False(t, tss.matchMatcher([]string{"https://example.com/foo/bar"}, exactMatcher("https://example.com/{foo}/bar")))
}

func TestMatchURLPattern(t *testing.T) {
	t.Parallel()

	tss, err := NewTopicSelectorStore(DefaultTopicSelectorStoreCacheSize)
	require.NoError(t, err)

	// Named group matching
	assert.True(t, tss.matchMatcher([]string{"https://example.com/books/123"}, urlPatternMatcher("https://example.com/books/:id")))
	assert.False(t, tss.matchMatcher([]string{"https://example.com/authors/123"}, urlPatternMatcher("https://example.com/books/:id")))

	// Wildcard path
	assert.True(t, tss.matchMatcher([]string{"https://example.com/a/b/c"}, urlPatternMatcher("https://example.com/*")))

	// Multiple named groups
	assert.True(t, tss.matchMatcher([]string{"https://example.com/users/42/posts/99"}, urlPatternMatcher("https://example.com/users/:uid/posts/:pid")))
	assert.False(t, tss.matchMatcher([]string{"https://example.com/users/42"}, urlPatternMatcher("https://example.com/users/:uid/posts/:pid")))

	// Multiple topics — at least one matches
	assert.True(t, tss.matchMatcher([]string{"https://example.com/authors/123", "https://example.com/books/123"}, urlPatternMatcher("https://example.com/books/:id")))

	// Case sensitivity: paths are case-sensitive, hosts are not (RFC 3986).
	assert.False(t, tss.matchMatcher([]string{"https://example.com/BOOKS/123"}, urlPatternMatcher("https://example.com/books/:id")))
	assert.True(t, tss.matchMatcher([]string{"https://EXAMPLE.com/books/123"}, urlPatternMatcher("https://example.com/books/:id")))

	// Match results are cached with a struct key (no collision possible). The
	// key is scoped to the base URL patterns were resolved against.
	_, found := tss.matchCache.GetIfPresent(matchCacheKey{
		Base:    tss.base(),
		Type:    MatcherTypeURLPattern,
		Pattern: "https://example.com/books/:id",
		Topics:  "https://example.com/books/123",
	})
	assert.True(t, found)
}

// TestMatchURLPatternRelative covers the spec case where both pattern and
// topic are relative — the shape the hub uses when it dispatches subscription
// events on `/.well-known/mercure/subscriptions/{matchType}/{match}/{subscriber}`
// topics. Relative ↔ relative must match; relative ↔ absolute must not.
func TestMatchURLPatternRelative(t *testing.T) {
	t.Parallel()

	tss, err := NewTopicSelectorStore(0)
	require.NoError(t, err)

	assert.True(t, tss.matchMatcher(
		[]string{"/.well-known/mercure/subscriptions/Exact/foo/bar"},
		urlPatternMatcher("/.well-known/mercure/subscriptions/Exact/:match/:subscriber"),
	))
	assert.True(t, tss.matchMatcher([]string{"/books/123"}, urlPatternMatcher("/books/:id")))
	assert.False(t, tss.matchMatcher([]string{"/authors/123"}, urlPatternMatcher("/books/:id")))

	// A relative pattern is anchored at the hub origin: an absolute topic on
	// a different origin must not match, and vice versa.
	assert.False(t, tss.matchMatcher([]string{"https://example.com/books/123"}, urlPatternMatcher("/books/:id")))
	assert.False(t, tss.matchMatcher([]string{"/books/123"}, urlPatternMatcher("https://example.com/books/:id")))
}

// TestMatchURLPatternConfiguredBase exercises the case the synthetic fallback
// cannot handle: a relative pattern matches an absolute topic on the hub URL
// (and vice-versa) when the store is configured with the real hub URL as base.
func TestMatchURLPatternConfiguredBase(t *testing.T) {
	t.Parallel()

	tss, err := NewTopicSelectorStore(0)
	require.NoError(t, err)
	require.NoError(t, tss.setBaseURL("https://hub.example.com"))

	assert.True(t, tss.matchMatcher([]string{"https://hub.example.com/books/123"}, urlPatternMatcher("/books/:id")))
	assert.True(t, tss.matchMatcher([]string{"/books/123"}, urlPatternMatcher("https://hub.example.com/books/:id")))
	assert.False(t, tss.matchMatcher([]string{"https://other.example.com/books/123"}, urlPatternMatcher("/books/:id")))
}

func TestValidatePattern(t *testing.T) {
	t.Parallel()

	tss, err := NewTopicSelectorStore(0)
	require.NoError(t, err)

	// Any string is a valid exact pattern.
	assert.NoError(t, tss.validatePattern(exactMatcher("{unclosed")))

	// Both absolute and relative URL patterns are accepted (relative ones
	// are anchored at the hub URL per the spec).
	assert.NoError(t, tss.validatePattern(urlPatternMatcher("https://example.com/books/:id")))
	assert.NoError(t, tss.validatePattern(urlPatternMatcher("*://example.com/books/:id")))
	assert.NoError(t, tss.validatePattern(urlPatternMatcher("/books/:id")))

	// Genuinely malformed patterns still fail.
	require.Error(t, tss.validatePattern(urlPatternMatcher("{unclosed")))

	// Unknown matcher types are rejected.
	assert.ErrorIs(t, tss.validatePattern(TopicMatcher{Type: "Regexp", Pattern: "fo+"}), ErrUnsupportedMatcherType)
}
