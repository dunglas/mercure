//go:build deprecated_topic

package mercure

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

// TestDeprecatedSubscriptionRoutesGatedOnCompat verifies the v8-style
// /subscriptions/{topic} collection route is only exposed when protocol
// compatibility is enabled, and 404s otherwise. The 2-segment single route
// cannot be tested this way because its shape collides with the modern
// /subscriptions/{matchType}/{match} collection route.
func TestDeprecatedSubscriptionRoutesGatedOnCompat(t *testing.T) {
	t.Parallel()

	// Modern hub — no compat → deprecated 1-segment route not registered.
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

	// Deprecated-compat hub — route registered; without a valid JWT the
	// handler answers 401 (not a mux 404), proving the route exists.
	deprecated, err := NewHub(t.Context(),
		WithAnonymous(),
		WithSubscriptions(),
		WithSubscriberJWT([]byte("subscriber"), "HS256"),
		WithProtocolVersionCompatibility(8),
	)
	require.NoError(t, err)

	t.Cleanup(func() { _ = deprecated.Stop(t.Context()) })

	req = httptest.NewRequest(http.MethodGet, defaultHubURL+subscriptionsPath+"/topic-A", nil)
	w = httptest.NewRecorder()
	deprecated.ServeHTTP(w, req)
	res = w.Result()
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode, "deprecated 1-segment route must exist under compat mode")
	require.NoError(t, res.Body.Close())
}
