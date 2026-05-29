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

	h := createDummy(t, withAllMatcherTypes()...)

	query := url.Values{"topic": {"https://example.com/foo", "https://example.com/{id}"}}
	matchers, err := h.parseMatchers(query, true)
	require.NoError(t, err)

	assert.Len(t, matchers, 2)
	assert.Equal(t, deprecatedMatcherTypeName, matchers[0].Type)
	assert.Equal(t, "https://example.com/foo", matchers[0].Pattern)
}

func TestParseMatchersCaseInsensitiveTopic(t *testing.T) {
	t.Parallel()

	h := createDummy(t, withAllMatcherTypes()...)

	// "TOPIC", "Topic" etc. should all be treated as the deprecated topic param
	query := url.Values{"TOPIC": {"foo"}}
	matchers, err := h.parseMatchers(query, true)
	require.NoError(t, err)

	assert.Len(t, matchers, 1)
}
