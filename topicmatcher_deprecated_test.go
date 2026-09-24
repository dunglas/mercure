//go:build deprecated_topic

package mercure

import (
	"strconv"
	"strings"
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
	assert.True(t, tms.matches([]string{"foo"}, deprecatedMatcher("foo")))
	assert.False(t, tms.matches([]string{"foo"}, deprecatedMatcher("bar")))
	assert.True(t, tms.matches([]string{"https://example.com/foo/bar"}, deprecatedMatcher("https://example.com/{foo}/bar")))
	assert.False(t, tms.matches([]string{"https://example.com/foo/bar/baz"}, deprecatedMatcher("https://example.com/{foo}/bar")))
	assert.True(t, tms.matches([]string{"https://example.com/kevin/dunglas"}, deprecatedMatcher("https://example.com/{firstname}/{lastname}")))
	assert.True(t, tms.matches([]string{"https://example.com/foo/bar"}, deprecatedMatcher("*")))

	// A selector that is not a valid URI Template falls back to exact-only.
	assert.False(t, tms.matches([]string{"foo"}, deprecatedMatcher("{invalid")))

	// So does one whose regexp exceeds Go's repeat limit, instead of panicking.
	tooManyVariables := "/{" + strings.Repeat("a,", 1100) + "a}"
	assert.False(t, tms.matches([]string{"/foo"}, deprecatedMatcher(tooManyVariables)))
	assert.True(t, tms.matches([]string{tooManyVariables}, deprecatedMatcher(tooManyVariables)))
	r, cached := tms.templateCache.GetIfPresent(tooManyVariables)
	require.True(t, cached, "failed compilations must not repeat on every publish")
	assert.Nil(t, r)
	assert.False(t, tms.matches([]string{"/bar"}, deprecatedMatcher(tooManyVariables)))

	// Template match results are cached, scoped to the resolution base URL.
	_, found := tms.matchCache.GetIfPresent(matchCacheKey{
		Base:    tms.base(),
		Type:    deprecatedMatcherTypeName,
		Pattern: "https://example.com/{foo}/bar",
		Topics:  "https://example.com/foo/bar",
	})
	assert.True(t, found)
}

func TestTemplateCacheBoundedByWeight(t *testing.T) {
	t.Parallel()

	tms, err := NewTopicMatcherStore(10_000)
	require.NoError(t, err)

	for i := range 20 {
		pattern := "/" + strconv.Itoa(i) + strings.Repeat("{+v}", 256)
		require.NotNil(t, tms.getRegexp(pattern))
	}

	// Counting entries would have retained all 20, some 4 MB under a 1 MB budget.
	assert.Less(t, tms.templateCache.EstimatedSize(), 20)
	assert.LessOrEqual(t, compiledCacheWeight(tms.templateCache), tms.compiledCacheWeight)
}
