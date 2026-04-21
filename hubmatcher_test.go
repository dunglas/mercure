package mercure

import (
	"net/http"
	"net/http/httptest"
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

func TestDefaultMatcherTypesDeprecated(t *testing.T) {
	t.Parallel()

	// In deprecated (v8) mode, defaults are Exact + URLPattern + URITemplate:
	// clients on either side of the migration keep working.
	h, err := NewHub(t.Context(), WithProtocolVersionCompatibility(8))
	require.NoError(t, err)

	defer h.Stop(t.Context()) //nolint:errcheck

	for _, name := range []string{"exact", "urlpattern", "uritemplate"} {
		_, ok := h.topicSelectorStore.ResolveMatcherType(name)
		assert.True(t, ok, "expected %s to be registered in deprecated mode", name)
	}
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

// TestDeprecatedSubscriptionRoutesGatedOnCompat verifies the v8-style
// /subscriptions/{topic} collection route is only exposed when protocol
// compatibility is enabled, and 404s otherwise. The 2-segment single route
// cannot be tested this way because its shape collides with the modern
// /subscriptions/{matchType}/{match} collection route.
func TestDeprecatedSubscriptionRoutesGatedOnCompat(t *testing.T) {
	t.Parallel()

	// Modern hub — no WithProtocolVersionCompatibility, legacy 1-segment route
	// not registered → mux-level 404. (createDummy forces v8 compat, so we
	// build the hub directly here.)
	modern, err := NewHub(t.Context(),
		WithAnonymous(),
		WithSubscriptions(),
		WithSubscriberJWT([]byte("subscriber"), "HS256"),
	)
	require.NoError(t, err)

	t.Cleanup(func() { _ = modern.Stop(t.Context()) })

	req := httptest.NewRequest(http.MethodGet, defaultHubURL+subscriptionsPath+"/topic-A", nil)
	w := httptest.NewRecorder()
	modern.ServeHTTP(w, req)
	res := w.Result()
	assert.Equal(t, http.StatusNotFound, res.StatusCode, "deprecated 1-segment route must 404 without compat mode")
	require.NoError(t, res.Body.Close())

	// Deprecated-compat hub — route registered; without a valid JWT we reach the
	// handler and get its 401 (not a mux 404), proving the route exists.
	deprecated := createDummy(t, WithSubscriptions(), WithAnonymous())
	req = httptest.NewRequest(http.MethodGet, defaultHubURL+subscriptionsPath+"/topic-A", nil)
	w = httptest.NewRecorder()
	deprecated.ServeHTTP(w, req)
	res = w.Result()
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode, "deprecated 1-segment route must exist under compat mode")
	require.NoError(t, res.Body.Close())
}
