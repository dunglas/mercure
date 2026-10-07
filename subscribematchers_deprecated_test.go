//go:build deprecated_topic && deprecated_claim

package mercure

import (
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMatchersDeprecatedTopic(t *testing.T) {
	t.Parallel()

	h := createDeprecatedDummy(t)

	query := url.Values{"topic": {"https://example.com/foo", "https://example.com/{id}"}}
	matchers, err := h.parseMatchers(query, true)
	require.NoError(t, err)

	assert.Len(t, matchers, 2)
	assert.Equal(t, deprecatedMatcherTypeName, matchers[0].Type)
	assert.Equal(t, "https://example.com/foo", matchers[0].Pattern)
	assert.Equal(t, "https://example.com/{id}", matchers[1].Pattern)
}

// TestParseMatchersURLPatternInCompatMode checks that the modern parameters
// remain available when compatibility mode is enabled.
func TestParseMatchersURLPatternInCompatMode(t *testing.T) {
	t.Parallel()

	h := createDeprecatedDummy(t)

	query := url.Values{
		"match_urlpattern": {"https://example.com/:id"},
		"topic":            {"https://example.com/{id}"},
	}
	matchers, err := h.parseMatchers(query, true)
	require.NoError(t, err)

	assert.Len(t, matchers, 2)
}

func TestParseMatchersDeprecatedTopicAggregateBudget(t *testing.T) {
	t.Parallel()

	tms, err := NewTopicMatcherStore(DefaultTopicMatcherStoreCacheSize)
	require.NoError(t, err)
	h := createDeprecatedDummy(t, WithTopicMatcherStore(tms))
	templates := make([]string, maxMatcherCount)

	for i := range templates {
		// Each template is valid and below the individual compilation limit.
		templates[i] = "/" + strconv.Itoa(i) + strings.Repeat("{a*}", 200)
		require.Less(t, templateCompileWeight(templates[i]), uint64(maxURLPatternWeight))
	}

	_, err = h.parseMatchers(url.Values{"topic": templates}, true)
	require.ErrorIs(t, err, errMatcherBudgetExceeded)

	for _, template := range templates {
		_, cached := tms.templateCache.GetIfPresent(template)
		assert.False(t, cached, "reject the entire request before compiling any template")
	}

	_, err = h.parseMatchers(url.Values{"topic": templates[:2]}, true)
	require.NoError(t, err, "ordinary multi-template subscriptions remain supported")

	_, err = h.parseMatchers(url.Values{"topic": templates[2:8], "match_urlpattern": {"https://example.com/" + strings.Repeat("a", maxPatternLength-20)}}, true)
	require.ErrorIs(t, err, errMatcherBudgetExceeded, "templates and URL Patterns share the budget")

	exact := make([]string, maxMatcherCount)
	for i := range exact {
		exact[i] = "https://example.com/" + strings.Repeat("a", maxPatternLength-20)
	}

	_, err = h.parseMatchers(url.Values{"topic": exact}, true)
	require.NoError(t, err, "selectors compared exactly do not consume the compilation budget")
}
