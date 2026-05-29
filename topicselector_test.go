package mercure

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTopicSelectorStoreRegistry(t *testing.T) {
	t.Parallel()

	tss := &TopicSelectorStore{}

	_, ok := tss.ResolveMatcherType("exact")
	assert.False(t, ok, "unregistered matcher must be absent")

	tss.RegisterMatcherType("Exact", ExactMatcher)

	for _, name := range []string{"exact", "EXACT", "Exact"} {
		mt, ok := tss.ResolveMatcherType(name)
		assert.True(t, ok, name)
		assert.Equal(t, ExactMatcher, mt, name)
	}
}

func TestRegisterMatcherTypeRejectsEmptyName(t *testing.T) {
	t.Parallel()

	tss := &TopicSelectorStore{}

	assert.Panics(t, func() { tss.RegisterMatcherType("", ExactMatcher) })
}

func TestRegisterMatcherTypeRejectsNilMatcher(t *testing.T) {
	t.Parallel()

	tss := &TopicSelectorStore{}

	assert.Panics(t, func() { tss.RegisterMatcherType("Exact", nil) })
}

func TestNewTopicMatcher(t *testing.T) {
	t.Parallel()

	tss := &TopicSelectorStore{}
	tss.RegisterMatcherType("Exact", ExactMatcher)

	m, err := tss.newTopicMatcher("Exact", "foo")
	require.NoError(t, err)
	assert.Equal(t, "Exact", m.Type)
	assert.Equal(t, "foo", m.Pattern)
	assert.Equal(t, ExactMatcher, m.matcher)

	// Case-insensitive lookup, but Type comes from the registered canonical name.
	m, err = tss.newTopicMatcher("exact", "bar")
	require.NoError(t, err)
	assert.Equal(t, "Exact", m.Type)

	// Unknown type
	_, err = tss.newTopicMatcher("Unknown", "baz")
	assert.ErrorIs(t, err, ErrUnsupportedMatcherType)
}

func TestMatchMatcher(t *testing.T) {
	t.Parallel()

	tss, err := NewTopicSelectorStore(DefaultTopicSelectorStoreCacheSize)
	require.NoError(t, err)

	tss.RegisterMatcherType("Exact", ExactMatcher)
	tss.RegisterMatcherType("URITemplate", URITemplateMatcher)

	// Exact matching
	exact, _ := tss.newTopicMatcher("Exact", "https://example.com/foo")
	assert.True(t, tss.matchMatcher([]string{"https://example.com/foo"}, exact))
	assert.False(t, tss.matchMatcher([]string{"https://example.com/bar"}, exact))

	// Wildcard
	wildcard, _ := tss.newTopicMatcher("Exact", "*")
	assert.True(t, tss.matchMatcher([]string{"anything"}, wildcard))

	// URI Template
	tmpl, _ := tss.newTopicMatcher("URITemplate", "https://example.com/{id}")
	assert.True(t, tss.matchMatcher([]string{"https://example.com/123"}, tmpl))
	assert.True(t, tss.matchMatcher([]string{"https://example.com/abc"}, tmpl))
	assert.False(t, tss.matchMatcher([]string{"https://example.com/123/sub"}, tmpl))
}

func TestMatchMatcherCaching(t *testing.T) {
	t.Parallel()

	tss, err := NewTopicSelectorStore(DefaultTopicSelectorStoreCacheSize)
	require.NoError(t, err)

	tss.RegisterMatcherType("URITemplate", URITemplateMatcher)
	tmpl, _ := tss.newTopicMatcher("URITemplate", "https://example.com/{id}")

	// First call computes and caches the result.
	assert.True(t, tss.matchMatcher([]string{"https://example.com/123"}, tmpl))

	_, found := tss.matchCache.GetIfPresent(matchCacheKey{
		Type:    "URITemplate",
		Pattern: "https://example.com/{id}",
		Topics:  "https://example.com/123",
	})
	assert.True(t, found, "cache entry missing")

	// Second call uses the cache.
	assert.True(t, tss.matchMatcher([]string{"https://example.com/123"}, tmpl))
}

func TestMatchMatcherCacheWithAlternateTopics(t *testing.T) {
	t.Parallel()

	tss, err := NewTopicSelectorStore(DefaultTopicSelectorStoreCacheSize)
	require.NoError(t, err)

	tss.RegisterMatcherType("URITemplate", URITemplateMatcher)
	tmpl, _ := tss.newTopicMatcher("URITemplate", "https://example.com/{id}")

	assert.True(t, tss.matchMatcher([]string{"https://example.com/a", "https://example.com/b"}, tmpl))

	_, found := tss.matchCache.GetIfPresent(matchCacheKey{
		Type:    "URITemplate",
		Pattern: "https://example.com/{id}",
		Topics:  "https://example.com/a" + topicsKeySeparator + "https://example.com/b",
	})
	assert.True(t, found, "cache entry for joined topics missing")
}

func TestMatchMatcherExactSkipsCache(t *testing.T) {
	t.Parallel()

	tss, err := NewTopicSelectorStore(DefaultTopicSelectorStoreCacheSize)
	require.NoError(t, err)

	tss.RegisterMatcherType("Exact", ExactMatcher)
	exact, _ := tss.newTopicMatcher("Exact", "foo")

	assert.True(t, tss.matchMatcher([]string{"foo"}, exact))

	_, found := tss.matchCache.GetIfPresent(matchCacheKey{Type: "Exact", Pattern: "foo", Topics: "foo"})
	assert.False(t, found, "Exact match should not be cached — it's already O(1)")
}

func TestMatchMatcherMultipleTopics(t *testing.T) {
	t.Parallel()

	tss := &TopicSelectorStore{}
	tss.RegisterMatcherType("Exact", ExactMatcher)

	exact, _ := tss.newTopicMatcher("Exact", "foo")

	// Multiple topics — at least one matches
	assert.True(t, tss.matchMatcher([]string{"bar", "foo"}, exact))
	assert.False(t, tss.matchMatcher([]string{"bar", "baz"}, exact))

	// Wildcard
	wildcard, _ := tss.newTopicMatcher("Exact", "*")
	assert.True(t, tss.matchMatcher([]string{"anything"}, wildcard))
}

func TestCustomMatcherType(t *testing.T) {
	t.Parallel()

	// Demonstrate that library users can create custom matcher types
	tss := &TopicSelectorStore{}
	tss.RegisterMatcherType("Prefix", prefixMatcher{})

	m, err := tss.newTopicMatcher("Prefix", "https://example.com/")
	require.NoError(t, err)

	assert.True(t, tss.matchMatcher([]string{"https://example.com/foo"}, m))
	assert.True(t, tss.matchMatcher([]string{"https://example.com/bar/baz"}, m))
	assert.False(t, tss.matchMatcher([]string{"https://other.com/foo"}, m))
}

// prefixMatcher is a test custom matcher type.
type prefixMatcher struct{}

func (prefixMatcher) Match(topics []string, pattern string) bool {
	for _, topic := range topics {
		if len(topic) >= len(pattern) && topic[:len(pattern)] == pattern {
			return true
		}
	}

	return false
}
