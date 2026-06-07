//go:build !deprecated_topic

package mercure

import (
	"net/http"
	"net/http/httptest"
	"testing"

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
