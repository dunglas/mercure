//go:build !deprecated_topic

package mercure

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSubscribeTopicCompatRequiresTag checks that enabling
// WithProtocolVersionCompatibility on a binary built without the
// deprecated_topic tag rejects v8 `topic` subscriptions instead of silently
// treating the selectors as Exact patterns.
func TestSubscribeTopicCompatRequiresTag(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t, WithProtocolVersionCompatibility(8))

	req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?topic=https://example.com/books/{id}", nil)
	w := httptest.NewRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() {
		require.NoError(t, resp.Body.Close())
	})

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Contains(t, w.Body.String(), "deprecated_topic")
}

// TestLegacyClaimRequiresTag checks that the legacy mercure claim is rejected
// on a binary built without the deprecated_claim tag, even under
// WithProtocolVersionCompatibility(8): such a token carries neither the
// at+jwt typ header nor an audience, so it fails RFC 9068 validation and the
// wildcard claim never authorizes anything.
func TestLegacyClaimRequiresTag(t *testing.T) {
	t.Parallel()

	hub := createDummy(t, WithProtocolVersionCompatibility(8))

	// A v8 mercure-claim token signed with the subscriber key, with no typ or
	// audience: the modern build must reject it.
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"mercure": map[string]any{"subscribe": []string{"*"}},
	})

	signed, err := token.SignedString([]byte("subscriber"))
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match_urlpattern=https://example.com/books/:id", nil)
	req.Header.Add("Authorization", bearerPrefix+signed)

	w := httptest.NewRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() {
		require.NoError(t, resp.Body.Close())
	})

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
