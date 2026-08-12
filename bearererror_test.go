package mercure

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testPublicURL        = "https://example.com/.well-known/mercure"
	wantResourceMetadata = "https://example.com/.well-known/oauth-protected-resource/.well-known/mercure"
)

func bearerErrorHub(tb testing.TB) *Hub {
	tb.Helper()

	return createDummy(tb, WithResourceIdentifier(testPublicURL))
}

func TestBearerChallengeNoToken(t *testing.T) {
	t.Parallel()

	hub := bearerErrorHub(t)

	req := httptest.NewRequest(http.MethodGet, defaultHubURL+subscriptionsPath, nil)
	w := httptest.NewRecorder()
	hub.SubscriptionsHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	challenge := resp.Header.Get("WWW-Authenticate")
	assert.True(t, strings.HasPrefix(challenge, "Bearer"), challenge)
	assert.NotContains(t, challenge, "error=")
	assert.Contains(t, challenge, `resource_metadata="`+wantResourceMetadata+`"`)
}

func TestBearerErrorInvalidToken(t *testing.T) {
	t.Parallel()

	hub := bearerErrorHub(t)

	valid := createDummyAuthorizedJWT(roleSubscriber, []string{"foo"})

	req := httptest.NewRequest(http.MethodGet, defaultHubURL+subscriptionsPath, nil)
	req.Header.Add("Authorization", bearerPrefix+valid[:len(valid)-6]+"123456")

	w := httptest.NewRecorder()
	hub.SubscriptionsHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("WWW-Authenticate"), `error="invalid_token"`)
}

func TestBearerErrorInsufficientScopeSubscriptionAPI(t *testing.T) {
	t.Parallel()

	hub := bearerErrorHub(t)

	req := httptest.NewRequest(http.MethodGet, defaultHubURL+subscriptionsPath, nil)
	req.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(roleSubscriber, []string{"https://example.com/not-the-api"}))

	w := httptest.NewRecorder()
	hub.SubscriptionsHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("WWW-Authenticate"), `error="insufficient_scope"`)
}

func TestBearerErrorInsufficientScopePublish(t *testing.T) {
	t.Parallel()

	hub := bearerErrorHub(t)

	form := url.Values{"topic": {"https://example.com/books/1"}, "private": {"on"}}
	req := httptest.NewRequest(http.MethodPost, defaultHubURL, strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(rolePublisher, []string{"https://example.com/other"}))

	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("WWW-Authenticate"), `error="insufficient_scope"`)
}

func TestBearerErrorInvalidRequestMalformedHeader(t *testing.T) {
	t.Parallel()

	hub := bearerErrorHub(t)

	req := httptest.NewRequest(http.MethodGet, defaultHubURL+subscriptionsPath, nil)
	req.Header.Add("Authorization", "Bearer x")

	w := httptest.NewRecorder()
	hub.SubscriptionsHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("WWW-Authenticate"), `error="invalid_request"`)
}
