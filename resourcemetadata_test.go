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
		// createDummy declares testIssuer as a plain trusted issuer (not
		// advertised); add an authorization server, the only issuer listed in
		// the metadata.
		WithIssuers([]Issuer{{
			Identifier:          "https://as.example.com",
			AuthorizationServer: true,
			Subscriber:          Static{Key: []byte("subscriber"), Algorithm: "HS256"},
		}}),
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
	assert.Equal(t, []string{"header"}, metadata.BearerMethodsSupported)
	assert.Equal(t, defaultCookieName, metadata.MercureCookie)
	assert.False(t, metadata.MercureSubscriptions)
	assert.Equal(t, []string{"https://as.example.com"}, metadata.AuthorizationServers)
	assert.Equal(t, []string{authorizationDetailTypeMercure}, metadata.AuthorizationDetailsTypesSupported)
}

func TestProtectedResourceMetadataAdvertisesCustomCookieName(t *testing.T) {
	hub := createDummy(t,
		WithResourceIdentifier("https://example.com/.well-known/mercure"),
		WithCookieName("__Secure-custom"),
	)

	req := httptest.NewRequest(http.MethodGet, protectedResourceMetadataPath, nil)
	w := httptest.NewRecorder()
	hub.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	var metadata protectedResourceMetadata
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&metadata))

	assert.Equal(t, "__Secure-custom", metadata.MercureCookie)
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

func TestProtectedResourceMetadataUsesResourceIdentifier(t *testing.T) {
	hub := createDummy(t, WithResourceIdentifier("https://example.com/.well-known/mercure"))

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

// With no static resource identifier configured, the metadata resource is
// derived from the request origin, so a hub reachable through several public
// URLs presents each caller the identity of the host it contacted.
func TestProtectedResourceMetadataDerivedFromRequest(t *testing.T) {
	tms, err := NewTopicMatcherStore(0)
	require.NoError(t, err)

	hub, err := NewHub(t.Context(), testIssuerOption(), WithTopicMatcherStore(tms))
	require.NoError(t, err)

	for _, host := range []string{"https://a.example.com", "https://b.example.com"} {
		req := httptest.NewRequest(http.MethodGet, host+protectedResourceMetadataPath, nil)
		w := httptest.NewRecorder()
		hub.ServeHTTP(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		var metadata protectedResourceMetadata
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&metadata))

		assert.Equal(t, host+"/.well-known/mercure", metadata.Resource)
	}
}

func TestProtectedResourceMetadataNotRegisteredWhenAnonymousWithoutKeys(t *testing.T) {
	tms, err := NewTopicMatcherStore(0)
	require.NoError(t, err)

	hub, err := NewHub(t.Context(), WithAnonymous(), WithTopicMatcherStore(tms))
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, protectedResourceMetadataPath, nil)
	w := httptest.NewRecorder()
	hub.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
