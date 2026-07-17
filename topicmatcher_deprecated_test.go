//go:build deprecated_topic

package mercure

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func deprecatedMatcher(pattern string) TopicMatcher {
	return TopicMatcher{Type: deprecatedMatcherTypeName, Pattern: pattern}
}

func TestMatchDeprecated(t *testing.T) {
	t.Parallel()

	tms, err := NewTopicMatcherStore(DefaultTopicMatcherStoreCacheSize)
	require.NoError(t, err)

	// v8 rules: exact comparison first, then URI Template fallback.
	assert.True(t, tms.matchMatcher([]string{"foo"}, deprecatedMatcher("foo")))
	assert.False(t, tms.matchMatcher([]string{"foo"}, deprecatedMatcher("bar")))
	assert.True(t, tms.matchMatcher([]string{"https://example.com/foo/bar"}, deprecatedMatcher("https://example.com/{foo}/bar")))
	assert.False(t, tms.matchMatcher([]string{"https://example.com/foo/bar/baz"}, deprecatedMatcher("https://example.com/{foo}/bar")))
	assert.True(t, tms.matchMatcher([]string{"https://example.com/kevin/dunglas"}, deprecatedMatcher("https://example.com/{firstname}/{lastname}")))
	assert.True(t, tms.matchMatcher([]string{"https://example.com/foo/bar"}, deprecatedMatcher("*")))

	// A selector that is not a valid URI Template falls back to exact-only.
	assert.False(t, tms.matchMatcher([]string{"foo"}, deprecatedMatcher("{invalid")))

	// Template match results are cached, scoped to the resolution base URL.
	_, found := tms.matchCache.GetIfPresent(matchCacheKey{
		Base:    tms.base(),
		Type:    deprecatedMatcherTypeName,
		Pattern: "https://example.com/{foo}/bar",
		Topics:  "https://example.com/foo/bar",
	})
	assert.True(t, found)
}
