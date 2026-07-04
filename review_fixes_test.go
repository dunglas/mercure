package mercure

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func publishAllDetails() []authorizationDetail {
	return []authorizationDetail{{
		Type:    authorizationDetailTypeMercure,
		Actions: []mercureAction{actionPublish},
		Topics:  []detailTopic{{TopicMatcher{Type: MatcherTypeExact, Pattern: "*"}}},
	}}
}

// TestPublishTopicWithNULRejected ensures a topic carrying a NUL byte is
// rejected with 400 before it can reach the shared match cache via the
// authorization grant check, where its NUL would collide with the topic-list
// key separator.
func TestPublishTopicWithNULRejected(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)
	token := mintAccessToken([]byte("publisher"), testResourceIdentifier, publishAllDetails())

	form := url.Values{}
	form.Add("topic", "https://example.com/a\x00https://example.com/b")
	form.Add("data", "x")

	req := httptest.NewRequest(http.MethodPost, defaultHubURL, strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", bearerPrefix+token)

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

// TestNonMercureAuthorizationDetailIgnored ensures a token carrying a
// non-"mercure" authorization detail with a foreign member shape (RFC 9396
// multi-resource token) does not reject the whole token: only mercure entries
// are validated.
func TestNonMercureAuthorizationDetailIgnored(t *testing.T) {
	t.Parallel()

	tss, err := NewTopicSelectorStore(0)
	require.NoError(t, err)

	var details []authorizationDetail
	require.NoError(t, json.Unmarshal([]byte(`[
		{"type":"tenant","topics":"acme","actions":{"read":true}},
		{"type":"mercure","actions":["subscribe"],"topics":[{"match":"https://example.com/foo"}]}
	]`), &details))

	authz, err := validateAuthorizationDetails(tss, details)
	require.NoError(t, err)
	assert.True(t, authz.grants(tss, actionSubscribe, "https://example.com/foo"))
}

// TestMercureDetailTopicMissingMatchRejected ensures a mercure detail topic
// without the required "match" property (or JSON null) invalidates the token.
func TestMercureDetailTopicMissingMatchRejected(t *testing.T) {
	t.Parallel()

	for name, payload := range map[string]string{
		"missing match": `[{"type":"mercure","actions":["subscribe"],"topics":[{"matchType":"Exact"}]}]`,
		"null topic":    `[{"type":"mercure","actions":["subscribe"],"topics":[null]}]`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var details []authorizationDetail
			err := json.Unmarshal([]byte(payload), &details)
			require.ErrorIs(t, err, errInvalidAuthorizationDetail)
		})
	}
}

// TestCompatModeEmptyResourceIdentifierAcceptsToken ensures a hub started in
// compatibility mode without a resource identifier (a build without the
// deprecated_claim tag) does not enforce an empty audience, which would reject
// every otherwise-valid access token.
func TestCompatModeEmptyResourceIdentifierAcceptsToken(t *testing.T) {
	t.Parallel()

	tss, err := NewTopicSelectorStore(0)
	require.NoError(t, err)

	h, err := NewHub(t.Context(),
		WithSubscriberJWT([]byte("subscriber"), jwt.SigningMethodHS256.Name),
		WithProtocolVersionCompatibility(7),
		WithTopicSelectorStore(tss),
	)
	require.NoError(t, err)
	require.Empty(t, h.resourceIdentifier)

	// A well-formed at+jwt token with an audience the hub does not know must
	// still be accepted, since no audience is enforced.
	token := mintAccessToken([]byte("subscriber"), "https://some.other.audience/", nil)

	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.Header.Add("Authorization", bearerPrefix+token)

	claims, err := h.authorize(r, false)
	require.NoError(t, err)
	require.NotNil(t, claims)
}
