package mercure

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPublishTopicWithNULRejected ensures a topic carrying a NUL byte is
// rejected with 400 before it can reach the shared match cache via canDispatch,
// where its NUL would collide with the topic-list key separator.
func TestPublishTopicWithNULRejected(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)

	form := url.Values{}
	form.Add("topic", "https://example.com/a\x00https://example.com/b")
	form.Add("data", "x")

	req := httptest.NewRequest(http.MethodPost, defaultHubURL, strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(rolePublisher, []string{"*"}))

	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()
	t.Cleanup(func() { assert.NoError(t, resp.Body.Close()) })

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// TestSubscribeInvalidURLPatternDoesNotLeakInternals ensures the 400 body for
// an uncompilable URL Pattern is a generic message, never the go-urlpattern
// error text (which can embed a live heap pointer, CWE-209).
func TestSubscribeInvalidURLPatternDoesNotLeakInternals(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t)

	req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?matchURLPattern=%2F%28unclosed", nil)
	w := httptest.NewRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()
	body := w.Body.String()
	t.Cleanup(func() { assert.NoError(t, resp.Body.Close()) })

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, errInvalidMatcherPattern.Error()+"\n", body)
	assert.NotContains(t, body, "0x")
	assert.NotContains(t, body, "urlpattern")
	assert.NotContains(t, body, "tokenizer")
}

// TestSubscribeMatcherClaimMissingMatchRejected ensures an object-form matcher
// claim without the required "match" property invalidates the token (401),
// rather than being silently accepted as an empty-pattern matcher.
func TestSubscribeMatcherClaimMissingMatchRejected(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"mercure": map[string]any{
			"subscribe": []any{map[string]any{"matchType": "Exact"}},
		},
	})
	tokenString, err := token.SignedString([]byte("subscriber"))
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match=https://example.com/foo", nil)
	req.Header.Add("Authorization", bearerPrefix+tokenString)

	w := httptest.NewRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()
	t.Cleanup(func() { assert.NoError(t, resp.Body.Close()) })

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// TestSharedStoreConflictingBaseURLRejected ensures a TopicSelectorStore shared
// across hubs configured with different public URLs is rejected at
// construction instead of silently corrupting relative-pattern matching.
func TestSharedStoreConflictingBaseURLRejected(t *testing.T) {
	t.Parallel()

	tss, err := NewTopicSelectorStore(DefaultTopicSelectorStoreCacheSize)
	require.NoError(t, err)

	_, err = NewHub(t.Context(),
		WithAnonymous(),
		WithTopicSelectorStore(tss),
		WithPublicURL("https://a.example.com/.well-known/mercure"),
	)
	require.NoError(t, err)

	_, err = NewHub(t.Context(),
		WithAnonymous(),
		WithTopicSelectorStore(tss),
		WithPublicURL("https://b.example.com/.well-known/mercure"),
	)
	require.ErrorIs(t, err, ErrConflictingBaseURL)
}

// TestSubscriptionAPIAuthorizationIgnoresQueryString ensures the subscription
// API authorizes against the absolute path only: a token presented via the
// authorization query parameter must not make the request-target differ from
// the Exact matcher the token grants.
func TestSubscriptionAPIAuthorizationIgnoresQueryString(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)

	token := createDummyAuthorizedJWT(roleSubscriber, []string{"/.well-known/mercure/subscriptions"})

	req := httptest.NewRequest(http.MethodGet, subscriptionsURL+"?authorization="+token, nil)
	w := httptest.NewRecorder()
	hub.SubscriptionsHandler(w, req)

	resp := w.Result()
	t.Cleanup(func() { assert.NoError(t, resp.Body.Close()) })

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestSubscriberSetMatchers ensures the exported construction path keeps the
// parallel matcher slices consistent, so external transports can rebuild a
// Subscriber programmatically.
func TestSubscriberSetMatchers(t *testing.T) {
	t.Parallel()

	tss, err := NewTopicSelectorStore(0)
	require.NoError(t, err)

	s := NewSubscriber(nil, tss)
	s.SetMatchers(
		[]TopicMatcher{{Type: MatcherTypeExact, Pattern: "https://example.com/foo"}},
		[]TopicMatcher{{Type: MatcherTypeExact, Pattern: "https://example.com/foo"}},
	)

	require.Len(t, s.EscapedMatchers, 1)
	assert.True(t, s.MatchTopics([]string{"https://example.com/foo"}, true))
	assert.False(t, s.MatchTopics([]string{"https://example.com/bar"}, false))
}
