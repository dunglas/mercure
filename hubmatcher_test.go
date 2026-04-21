package mercure

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withAllMatcherTypes is a test helper that registers every built-in matcher type.
// Each test that needs them all applies it to keep the production API minimal.
func withAllMatcherTypes() []Option {
	return []Option{
		WithMatcherType("Exact", ExactMatcher),
		WithMatcherType("URITemplate", URITemplateMatcher),
		WithMatcherType("URLPattern", URLPatternMatcher),
		WithMatcherType("Regexp", RegexpMatcher),
		WithMatcherType("CEL", CELMatcher),
	}
}

func TestWithMatcherType(t *testing.T) {
	t.Parallel()

	h, err := NewHub(t.Context(),
		WithMatcherType("Exact", ExactMatcher),
		WithMatcherType("URLPattern", URLPatternMatcher),
	)
	require.NoError(t, err)

	defer h.Stop(t.Context()) //nolint:errcheck

	// Registered types should be available
	_, ok := h.topicSelectorStore.ResolveMatcherType("exact")
	assert.True(t, ok)
	_, ok = h.topicSelectorStore.ResolveMatcherType("urlpattern")
	assert.True(t, ok)

	// Not registered types should not be available
	_, ok = h.topicSelectorStore.ResolveMatcherType("regexp")
	assert.False(t, ok)
	_, ok = h.topicSelectorStore.ResolveMatcherType("cel")
	assert.False(t, ok)

	// Exact is always present even if not explicitly registered
	_, ok = h.topicSelectorStore.ResolveMatcherType("exact")
	assert.True(t, ok)
}

func TestDefaultMatcherTypesModern(t *testing.T) {
	t.Parallel()

	// In modern mode, defaults are Exact + URLPattern.
	h, err := NewHub(t.Context())
	require.NoError(t, err)

	defer h.Stop(t.Context()) //nolint:errcheck

	_, ok := h.topicSelectorStore.ResolveMatcherType("exact")
	assert.True(t, ok)
	_, ok = h.topicSelectorStore.ResolveMatcherType("urlpattern")
	assert.True(t, ok)

	_, ok = h.topicSelectorStore.ResolveMatcherType("uritemplate")
	assert.False(t, ok, "URITemplate must not be registered by default outside deprecated mode")
}

func TestWithCustomMatcherType(t *testing.T) {
	t.Parallel()

	h, err := NewHub(t.Context(),
		WithMatcherType("Prefix", prefixMatcher{}),
	)
	require.NoError(t, err)

	defer h.Stop(t.Context()) //nolint:errcheck

	// Custom matcher registered
	_, ok := h.topicSelectorStore.ResolveMatcherType("prefix")
	assert.True(t, ok)

	// Exact is always present
	_, ok = h.topicSelectorStore.ResolveMatcherType("exact")
	assert.True(t, ok)
}

func TestWithProtocolVersionCompatibility8(t *testing.T) {
	t.Parallel()

	h, err := NewHub(t.Context(), WithProtocolVersionCompatibility(8))
	require.NoError(t, err)

	defer h.Stop(t.Context()) //nolint:errcheck

	assert.True(t, h.isBackwardCompatiblyEnabledWith(8))
	assert.True(t, h.isBackwardCompatiblyEnabledWith(9))
	assert.False(t, h.isBackwardCompatiblyEnabledWith(7))
}
