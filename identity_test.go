package mercure

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// With no static resource identifier, the expected audience is derived from the
// public URL the client contacted, so a token minted for one host authorizes
// there and is rejected on another.
func TestAuthorizeDerivesAudienceFromRequestHost(t *testing.T) {
	t.Parallel()

	tms, err := NewTopicMatcherStore(0)
	require.NoError(t, err)

	hub, err := NewHub(t.Context(), testIssuerOption(), WithTopicMatcherStore(tms))
	require.NoError(t, err)

	details := []authorizationDetail{{
		Type:    authorizationDetailTypeMercure,
		Actions: []mercureAction{actionSubscribe},
		Topics:  stringsToDetailTopics([]string{"https://example.com/books/1"}),
	}}
	token := mintAccessToken([]byte("subscriber"), "https://a.example.com/.well-known/mercure", details)

	// Accepted on the host the token's aud names.
	r := httptest.NewRequest(http.MethodGet, "https://a.example.com"+defaultHubURL, nil)
	r.Header.Set("Authorization", bearerPrefix+token)
	c, err := hub.authorize(r, false)
	require.NoError(t, err)
	assert.NotNil(t, c)

	// Rejected on a different host: the derived audience no longer matches.
	r = httptest.NewRequest(http.MethodGet, "https://b.example.com"+defaultHubURL, nil)
	r.Header.Set("Authorization", bearerPrefix+token)
	_, err = hub.authorize(r, false)
	require.ErrorIs(t, err, ErrInvalidJWT)
}

// A request whose origin is not in the public-URL allowlist is rejected with
// 421 before any identity is derived from it; an allowed origin passes the
// guard. The scheme is pinned: an https allowlist rejects an http request.
func TestServeHTTPRejectsOriginNotInAllowlist(t *testing.T) {
	t.Parallel()

	hub := createDummy(t, WithPublicURLs([]string{"https://allowed.example.com"}))

	for _, target := range []string{
		"https://denied.example.com" + defaultHubURL,
		"http://allowed.example.com" + defaultHubURL, // right host, wrong scheme
	} {
		r := httptest.NewRequest(http.MethodGet, target, nil)
		w := httptest.NewRecorder()
		hub.ServeHTTP(w, r)
		assert.Equal(t, http.StatusMisdirectedRequest, w.Result().StatusCode, target)
	}

	r := httptest.NewRequest(http.MethodGet, "https://allowed.example.com"+defaultHubURL, nil)
	w := httptest.NewRecorder()
	hub.ServeHTTP(w, r)
	assert.NotEqual(t, http.StatusMisdirectedRequest, w.Result().StatusCode)
}
