//go:build deprecated_topic

package mercure

import (
	"net/url"
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
		"matchURLPattern": {"https://example.com/:id"},
		"topic":           {"https://example.com/{id}"},
	}
	matchers, err := h.parseMatchers(query, true)
	require.NoError(t, err)

	assert.Len(t, matchers, 2)
}
