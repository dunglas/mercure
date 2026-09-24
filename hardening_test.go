package mercure

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

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

// TestUpdateValidateRejectsControlCharsInID ensures the id and type fields
// reject all control characters (not only CR/LF/NUL) and invalid UTF-8, so an
// event id cannot carry a DEL or C1 byte into the Last-Event-ID header.
func TestUpdateValidateRejectsControlCharsInID(t *testing.T) {
	t.Parallel()

	const topic = "https://example.com/1"

	require.ErrorIs(t, (&Update{Topics: []string{topic}, ID: "a\x7fb"}).Validate(urlPatternFallbackBase), ErrInvalidEventID)
	require.ErrorIs(t, (&Update{Topics: []string{topic}, Type: "a\x9fb"}).Validate(urlPatternFallbackBase), ErrInvalidEventType)
	require.ErrorIs(t, (&Update{Topics: []string{topic}, ID: "a\xffb"}).Validate(urlPatternFallbackBase), ErrInvalidEventID) // invalid UTF-8
	require.NoError(t, (&Update{Topics: []string{topic}, ID: topic, Type: "message"}).Validate(urlPatternFallbackBase))
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

// The protocol requires rejecting requests exceeding the hub's body-size
// limit with a 413 status code.
func TestPublishBodyOverLimitRejectedWith413(t *testing.T) {
	t.Parallel()

	hub := createDummy(t, WithMaxRequestBodySize(1024))
	token := mintAccessToken([]byte("publisher"), testResourceIdentifier, publishAllDetails())

	form := url.Values{}
	form.Add("topic", "https://example.com/books/1")
	form.Add("data", strings.Repeat("x", 2048))

	req := httptest.NewRequest(http.MethodPost, defaultHubURL, strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", bearerPrefix+token)

	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() { assert.NoError(t, resp.Body.Close()) })

	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)
}

func TestQuerySubscribeBodyOverLimitRejectedWith413(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t, WithMaxRequestBodySize(1024))

	body := "match=" + strings.Repeat("a", 2048)
	req := httptest.NewRequest(methodQuery, defaultHubURL, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() { assert.NoError(t, resp.Body.Close()) })

	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)
}

// A zero size disables the in-hub limit (delegation to a reverse proxy).
func TestMaxRequestBodySizeZeroDisablesLimit(t *testing.T) {
	t.Parallel()

	hub := createDummy(t, WithMaxRequestBodySize(0))
	token := mintAccessToken([]byte("publisher"), testResourceIdentifier, publishAllDetails())

	form := url.Values{}
	form.Add("topic", "https://example.com/books/1")
	form.Add("data", strings.Repeat("x", int(DefaultMaxRequestBodySize)+1))

	req := httptest.NewRequest(http.MethodPost, defaultHubURL, strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", bearerPrefix+token)

	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() { assert.NoError(t, resp.Body.Close()) })

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestSubscribeInvalidURLPatternDoesNotLeakInternals ensures the 400 body for
// an uncompilable URL Pattern is a generic message, never the go-urlpattern
// error text (which can embed a live heap pointer, CWE-209).
func TestSubscribeInvalidURLPatternDoesNotLeakInternals(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t)

	req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match_urlpattern=%2F%28unclosed", nil)
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

// TestSharedStoreConflictingBaseURLRejected ensures a TopicMatcherStore shared
// across hubs configured with different public URLs is rejected at
// construction instead of silently corrupting relative-pattern matching.
func TestSharedStoreConflictingBaseURLRejected(t *testing.T) {
	t.Parallel()

	tms, err := NewTopicMatcherStore(DefaultTopicMatcherStoreCacheSize)
	require.NoError(t, err)

	_, err = NewHub(t.Context(),
		WithAnonymous(),
		WithTopicMatcherStore(tms),
		WithResourceIdentifier("https://a.example.com/.well-known/mercure"),
	)
	require.NoError(t, err)

	_, err = NewHub(t.Context(),
		WithAnonymous(),
		WithTopicMatcherStore(tms),
		WithResourceIdentifier("https://b.example.com/.well-known/mercure"),
	)
	require.ErrorIs(t, err, ErrConflictingBaseURL)
}

// TestSubscriberSetMatchers ensures the exported construction path keeps the
// parallel matcher slices consistent, so external transports can rebuild a
// Subscriber programmatically.
func TestSubscriberSetMatchers(t *testing.T) {
	t.Parallel()

	tms, err := NewTopicMatcherStore(0)
	require.NoError(t, err)

	s := NewSubscriber(nil, tms)
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

	tms, err := NewTopicMatcherStore(0)
	require.NoError(t, err)

	var details []authorizationDetail
	require.NoError(t, json.Unmarshal([]byte(`[
		{"type":"tenant","topics":"acme","actions":{"read":true}},
		{"type":"https://mercure.rocks/authorization-detail","actions":["subscribe"],"topics":[{"match":"https://example.com/foo"}]}
	]`), &details))

	authz, err := validateAuthorizationDetails(tms, details)
	require.NoError(t, err)
	assert.True(t, authz.grants(tms, actionSubscribe, "https://example.com/foo"))
}

// TestMercureDetailTopicMissingMatchRejected ensures a mercure detail topic
// without the required "match" property (or JSON null) invalidates the token.
func TestMercureDetailTopicMissingMatchRejected(t *testing.T) {
	t.Parallel()

	for name, payload := range map[string]string{
		"missing match": `[{"type":"https://mercure.rocks/authorization-detail","actions":["subscribe"],"topics":[{"match_type":"exact"}]}]`,
		"null topic":    `[{"type":"https://mercure.rocks/authorization-detail","actions":["subscribe"],"topics":[null]}]`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var details []authorizationDetail

			err := json.Unmarshal([]byte(payload), &details)
			require.ErrorIs(t, err, errInvalidAuthorizationDetail)
		})
	}
}

// TestMercureDetailEmptyPatternRejected ensures a detail topic with an empty
// match pattern invalidates the token rather than being accepted as a
// no-op matcher.
func TestMercureDetailEmptyPatternRejected(t *testing.T) {
	t.Parallel()

	tms, err := NewTopicMatcherStore(0)
	require.NoError(t, err)

	var details []authorizationDetail
	require.NoError(t, json.Unmarshal([]byte(`[{"type":"https://mercure.rocks/authorization-detail","actions":["subscribe"],"topics":[{"match":""}]}]`), &details))

	_, err = validateAuthorizationDetails(tms, details)
	require.ErrorIs(t, err, errInvalidAuthorizationDetail)
}

// TestSubscriptionPayloadFastPath ensures the no-payload short-circuit still
// resolves per-subscription payloads to the correct values.
func TestSubscriptionPayloadFastPath(t *testing.T) {
	t.Parallel()

	tms, err := NewTopicMatcherStore(0)
	require.NoError(t, err)

	// No detail carries a payload: every subscription payload is nil.
	s := NewSubscriber(nil, tms)
	s.Claims = detailClaims(t, tms, subscribeDetail(nil, TopicMatcher{Type: MatcherTypeExact, Pattern: "https://example.com/a"}))
	s.SetMatchers([]TopicMatcher{{Type: MatcherTypeExact, Pattern: "https://example.com/a"}}, nil)
	require.Len(t, s.SubscriptionPayloads, 1)
	assert.Nil(t, s.SubscriptionPayloads[0])

	// A detail carries a payload: the matching subscription resolves to it.
	s2 := NewSubscriber(nil, tms)
	s2.Claims = detailClaims(t, tms, subscribeDetail(map[string]any{"k": "v"}, TopicMatcher{Type: MatcherTypeExact, Pattern: "https://example.com/a"}))
	s2.SetMatchers([]TopicMatcher{{Type: MatcherTypeExact, Pattern: "https://example.com/a"}}, nil)
	require.Len(t, s2.SubscriptionPayloads, 1)
	assert.Equal(t, map[string]any{"k": "v"}, s2.SubscriptionPayloads[0])
}

// TestInvalidBaseURLRejected ensures a non-absolute public URL is rejected at
// configuration time rather than surfacing as an opaque per-request error.
func TestInvalidBaseURLRejected(t *testing.T) {
	t.Parallel()

	tms, err := NewTopicMatcherStore(0)
	require.NoError(t, err)

	require.ErrorIs(t, tms.setBaseURL("not-a-url"), ErrInvalidBaseURL)
	require.ErrorIs(t, tms.setBaseURL("/relative/only"), ErrInvalidBaseURL)
	require.NoError(t, tms.setBaseURL("https://hub.example.com/.well-known/mercure"))
}

// TestValidProtocolStringRejectsFormatChars ensures invisible Unicode format
// characters (bidirectional / zero-width controls, category Cf) are rejected in
// topics/matchers/ids -- the Trojan-Source spoofing vector -- while RTL script
// letters remain valid.
func TestValidProtocolStringRejectsFormatChars(t *testing.T) {
	t.Parallel()

	assert.False(t, validProtocolString("a\u202eb")) // RIGHT-TO-LEFT OVERRIDE
	assert.False(t, validProtocolString("a\u200bb")) // ZERO WIDTH SPACE
	assert.False(t, validProtocolString("a\ufeffb")) // ZERO WIDTH NO-BREAK SPACE
	assert.True(t, validProtocolString("https://example.com/foo"))
	assert.True(t, validProtocolString("https://example.com/\u0645\u0631\u062d\u0628\u0627")) // RTL letters are fine
}

// signRawClaims signs a hand-written claim set, which may repeat members.
func signRawClaims(tb testing.TB, key []byte, payload string) string {
	tb.Helper()

	enc := base64.RawURLEncoding
	signingString := enc.EncodeToString([]byte(`{"alg":"HS256","typ":"at+jwt"}`)) + "." + enc.EncodeToString([]byte(payload))

	sig, err := jwt.SigningMethodHS256.Sign(signingString, key)
	require.NoError(tb, err)

	return signingString + "." + enc.EncodeToString(sig)
}

// Claim names are case-sensitive and unique (RFC 7519 §4).
func TestAuthorizeAmbiguousClaimNames(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)
	past, future := time.Now().Add(-time.Hour).Unix(), time.Now().Add(time.Hour).Unix()
	grant := `[{"type":"https://mercure.rocks/authorization-detail","actions":["subscribe"],"topics":[{"match":"*"}]}]`
	registered := fmt.Sprintf(`"iss":%q,"aud":%q`, testIssuer, testResourceIdentifier)

	authorize := func(t *testing.T, payload string) (*claims, error) {
		t.Helper()

		r := httptest.NewRequest(http.MethodGet, defaultHubURL, nil)
		r.Header.Add("Authorization", bearerPrefix+signRawClaims(t, []byte("subscriber"), payload))

		return hub.authorize(r, false)
	}

	for name, payload := range map[string]string{
		"case-folded exp": fmt.Sprintf(`{%s,"exp":%d,"EXP":%d,"authorization_details":%s}`, registered, past, future, grant),
		"case-folded aud": fmt.Sprintf(`{"iss":%q,"aud":"https://other.example/.well-known/mercure","Aud":%q,"exp":%d,"authorization_details":%s}`, testIssuer, testResourceIdentifier, future, grant),
		"duplicate exp":   fmt.Sprintf(`{%s,"exp":%d,"exp":%d,"authorization_details":%s}`, registered, past, future, grant),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := authorize(t, payload)
			require.ErrorIs(t, err, ErrInvalidJWT)
		})
	}

	t.Run("case-folded authorization_details", func(t *testing.T) {
		t.Parallel()

		c, err := authorize(t, fmt.Sprintf(`{%s,"exp":%d,"AUTHORIZATION_DETAILS":%s}`, registered, future, grant))
		require.NoError(t, err)
		assert.Empty(t, c.AuthorizationDetails)
	})
}
