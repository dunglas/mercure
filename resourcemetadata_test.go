package mercure

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProtectedResourceMetadata(t *testing.T) {
	hub := createDummy(t,
		WithResourceIdentifier("https://example.com/.well-known/mercure"),
		WithAuthorizationServers([]string{"https://as.example.com"}),
	)

	req := httptest.NewRequest(http.MethodGet, protectedResourceMetadataPath, nil)
	w := httptest.NewRecorder()
	hub.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var metadata protectedResourceMetadata
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&metadata))

	assert.Equal(t, "https://example.com/.well-known/mercure", metadata.Resource)
	assert.Equal(t, []string{"header", "query"}, metadata.BearerMethodsSupported)
	assert.True(t, metadata.MercureCookie)
	assert.False(t, metadata.MercureSubscriptions)
	assert.Equal(t, []string{"https://as.example.com"}, metadata.AuthorizationServers)
	assert.Equal(t, []string{authorizationDetailTypeMercure}, metadata.AuthorizationDetailsTypesSupported)
}

func TestProtectedResourceMetadataAdvertisesSubscriptions(t *testing.T) {
	hub := createDummy(t,
		WithResourceIdentifier("https://example.com/.well-known/mercure"),
		WithSubscriptions(),
	)

	req := httptest.NewRequest(http.MethodGet, protectedResourceMetadataPath, nil)
	w := httptest.NewRecorder()
	hub.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	var metadata protectedResourceMetadata
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&metadata))

	assert.True(t, metadata.MercureSubscriptions)
}

func TestProtectedResourceMetadataDefaultsToPublicURL(t *testing.T) {
	hub := createDummy(t, WithPublicURL("https://example.com/.well-known/mercure"))

	req := httptest.NewRequest(http.MethodGet, protectedResourceMetadataPath, nil)
	w := httptest.NewRecorder()
	hub.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	var metadata protectedResourceMetadata
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&metadata))

	assert.Equal(t, "https://example.com/.well-known/mercure", metadata.Resource)
	assert.Empty(t, metadata.AuthorizationServers)
}

func TestProtectedResourceMetadataNotRegisteredWhenAnonymousWithoutKeys(t *testing.T) {
	tss, err := NewTopicSelectorStore(0)
	require.NoError(t, err)

	hub, err := NewHub(t.Context(), WithAnonymous(), WithTopicSelectorStore(tss))
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, protectedResourceMetadataPath, nil)
	w := httptest.NewRecorder()
	hub.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
