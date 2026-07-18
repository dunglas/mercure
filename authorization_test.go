//go:build !deprecated_claim

package mercure

import (
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// signSubscriberToken signs claims with the dummy subscriber key.
func signSubscriberToken(tb testing.TB, c *claims) string {
	tb.Helper()

	token := jwt.New(jwt.SigningMethodHS256)
	token.Header["typ"] = atJWTType
	token.Claims = c

	s, err := token.SignedString([]byte("subscriber"))
	require.NoError(tb, err)

	return s
}

func subscriberRegisteredClaims() jwt.RegisteredClaims {
	return jwt.RegisteredClaims{
		Issuer:    testIssuer,
		Audience:  jwt.ClaimStrings{testResourceIdentifier},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
}

func TestAuthorizeMultipleAuthorizationHeader(t *testing.T) {
	t.Parallel()

	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.Header.Add("Authorization", bearerPrefix+"x")
	r.Header.Add("Authorization", bearerPrefix+"y")

	h := createDummy(t)

	claims, err := h.authorize(r, false)
	require.ErrorIs(t, err, ErrInvalidAuthorizationHeader)
	require.Nil(t, claims)
}

func TestAuthorizeAuthorizationHeaderTooShort(t *testing.T) {
	t.Parallel()

	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.Header.Add("Authorization", "Bearer x")

	h := createDummy(t)

	claims, err := h.authorize(r, false)
	require.ErrorIs(t, err, ErrInvalidAuthorizationHeader)
	require.Nil(t, claims)
}

func TestAuthorizeAuthorizationHeaderNoBearer(t *testing.T) {
	t.Parallel()

	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.Header.Add("Authorization", "Greater "+createDummyAuthorizedJWT(roleSubscriber, nil))

	h := createDummy(t)

	claims, err := h.authorize(r, false)
	require.ErrorIs(t, err, ErrInvalidAuthorizationHeader)
	require.Nil(t, claims)
}

func TestAuthorizeAuthorizationHeaderInvalidAlg(t *testing.T) {
	t.Parallel()

	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.Header.Add("Authorization", bearerPrefix+createDummyNoneSignedJWT())

	h := createDummy(t)

	claims, err := h.authorize(r, false)
	require.Error(t, err)
	require.Nil(t, claims)
}

func TestAuthorizeAuthorizationHeaderInvalidSignature(t *testing.T) {
	t.Parallel()

	valid := createDummyAuthorizedJWT(roleSubscriber, []string{"foo"})

	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.Header.Add("Authorization", bearerPrefix+valid[:len(valid)-8]+"12345678")

	h := createDummy(t)

	claims, err := h.authorize(r, false)
	require.Error(t, err)
	require.Nil(t, claims)
}

func TestAuthorizeAuthorizationHeader(t *testing.T) {
	t.Parallel()

	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(roleSubscriber, []string{"foo", "bar"}))

	h := createDummy(t)

	claims, err := h.authorize(r, false)
	require.NoError(t, err)
	assert.True(t, claims.authz.grants(h.topicMatcherStore, actionSubscribe, "foo"))
	assert.True(t, claims.authz.grants(h.topicMatcherStore, actionSubscribe, "bar"))
	assert.False(t, claims.authz.grants(h.topicMatcherStore, actionSubscribe, "baz"))
}

func TestAuthorizeAuthorizationHeaderLowercaseScheme(t *testing.T) {
	t.Parallel()

	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	// The auth scheme is case-insensitive (RFC 9110 §11.1).
	r.Header.Add("Authorization", "bearer "+createDummyAuthorizedJWT(roleSubscriber, []string{"foo"}))

	h := createDummy(t)

	claims, err := h.authorize(r, false)
	require.NoError(t, err)
	assert.True(t, claims.authz.grants(h.topicMatcherStore, actionSubscribe, "foo"))
}

// The RFC 6750 "access_token" query parameter is not a supported mechanism
// (RFC 9700 §4.3.2): a valid token passed this way falls through to anonymous.
func TestAuthorizeAccessTokenQueryIgnored(t *testing.T) {
	t.Parallel()

	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	query := r.URL.Query()
	query.Set("access_token", createDummyAuthorizedJWT(roleSubscriber, []string{"foo"}))
	r.URL.RawQuery = query.Encode()

	h := createAnonymousDummy(t)

	claims, err := h.authorize(r, false)
	require.NoError(t, err)
	require.Nil(t, claims)
}

// The deprecated "authorization" query parameter is ignored in modern mode,
// falling through to anonymous access.
func TestAuthorizeLegacyQueryParamIgnored(t *testing.T) {
	t.Parallel()

	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	query := r.URL.Query()
	query.Set("authorization", createDummyAuthorizedJWT(roleSubscriber, []string{"foo"}))
	r.URL.RawQuery = query.Encode()

	h := createAnonymousDummy(t)

	claims, err := h.authorize(r, false)
	require.NoError(t, err)
	require.Nil(t, claims)
}

func TestAuthorizeCookie(t *testing.T) {
	t.Parallel()

	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.AddCookie(&http.Cookie{Name: defaultCookieName, Value: createDummyAuthorizedJWT(roleSubscriber, []string{"foo"})})

	h := createDummy(t)

	claims, err := h.authorize(r, false)
	require.NoError(t, err)
	assert.True(t, claims.authz.grants(h.topicMatcherStore, actionSubscribe, "foo"))
}

// A modern hub does not honor the pre-1.0 "mercureAuthorization" cookie name,
// so a client still using it is treated as anonymous.
func TestAuthorizeLegacyCookieNameIgnored(t *testing.T) {
	t.Parallel()

	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(roleSubscriber, []string{"foo"})})

	h := createDummy(t)

	claims, err := h.authorize(r, false)
	require.NoError(t, err)
	require.Nil(t, claims)
}

func TestAuthorizeCookieNoOriginNoReferer(t *testing.T) {
	t.Parallel()

	r, _ := http.NewRequest(http.MethodPost, defaultHubURL, nil)
	r.AddCookie(&http.Cookie{Name: defaultCookieName, Value: createDummyAuthorizedJWT(rolePublisher, []string{"foo"})})

	h := createDummy(t)

	claims, err := h.authorize(r, true)
	require.ErrorIs(t, err, ErrNoOrigin)
	require.Nil(t, claims)
}

func TestAuthorizeCookieOriginNotAllowed(t *testing.T) {
	t.Parallel()

	r, _ := http.NewRequest(http.MethodPost, defaultHubURL, nil)
	r.Header.Add("Origin", "https://example.com")
	r.AddCookie(&http.Cookie{Name: defaultCookieName, Value: createDummyAuthorizedJWT(rolePublisher, []string{"foo"})})

	h := createDummy(t, WithPublishOrigins([]string{"https://example.net"}))

	claims, err := h.authorize(r, true)
	require.ErrorIs(t, err, ErrOriginNotAllowed)
	require.Nil(t, claims)
}

// RFC 9068: a token whose typ header is not at+jwt is rejected in modern mode.
func TestAuthorizeRejectsNonAccessToken(t *testing.T) {
	t.Parallel()

	token := jwt.New(jwt.SigningMethodHS256) // default typ "JWT"
	token.Claims = &claims{
		RegisteredClaims:     subscriberRegisteredClaims(),
		AuthorizationDetails: subscribeDetailsFromMatchers(nil, TopicMatcher{Type: MatcherTypeExact, Pattern: "foo"}),
	}
	s, err := token.SignedString([]byte("subscriber"))
	require.NoError(t, err)

	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.Header.Add("Authorization", bearerPrefix+s)

	h := createDummy(t)

	claims, err := h.authorize(r, false)
	require.ErrorIs(t, err, ErrInvalidJWT)
	require.Nil(t, claims)
}

// RFC 9068 media types are case-insensitive, including the optional
// "application/" prefix.
func TestAuthorizeAcceptsMixedCaseATJWTType(t *testing.T) {
	t.Parallel()

	for _, typ := range []string{"at+jwt", "AT+JWT", "application/at+jwt", "Application/AT+JWT"} {
		token := jwt.New(jwt.SigningMethodHS256)
		token.Header["typ"] = typ
		token.Claims = &claims{
			RegisteredClaims:     subscriberRegisteredClaims(),
			AuthorizationDetails: subscribeDetailsFromMatchers(nil, TopicMatcher{Type: MatcherTypeExact, Pattern: "foo"}),
		}

		s, err := token.SignedString([]byte("subscriber"))
		require.NoError(t, err)

		r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
		r.Header.Add("Authorization", bearerPrefix+s)

		h := createDummy(t)

		c, err := h.authorize(r, false)
		require.NoError(t, err, typ)
		require.NotNil(t, c, typ)
	}
}

func TestAuthorizeRejectsWrongAudience(t *testing.T) {
	t.Parallel()

	c := &claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Audience:  jwt.ClaimStrings{"https://other.example.com/.well-known/mercure"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		AuthorizationDetails: subscribeDetailsFromMatchers(nil, TopicMatcher{Type: MatcherTypeExact, Pattern: "foo"}),
	}

	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.Header.Add("Authorization", bearerPrefix+signSubscriberToken(t, c))

	h := createDummy(t)

	claims, err := h.authorize(r, false)
	require.Error(t, err)
	require.Nil(t, claims)
}

func TestAuthorizeRejectsMissingAudience(t *testing.T) {
	t.Parallel()

	c := &claims{
		RegisteredClaims:     jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
		AuthorizationDetails: subscribeDetailsFromMatchers(nil, TopicMatcher{Type: MatcherTypeExact, Pattern: "foo"}),
	}

	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.Header.Add("Authorization", bearerPrefix+signSubscriberToken(t, c))

	h := createDummy(t)

	claims, err := h.authorize(r, false)
	require.Error(t, err)
	require.Nil(t, claims)
}

func TestAuthorizeRejectsMissingExpiration(t *testing.T) {
	t.Parallel()

	c := &claims{
		RegisteredClaims:     jwt.RegisteredClaims{Audience: jwt.ClaimStrings{testResourceIdentifier}},
		AuthorizationDetails: subscribeDetailsFromMatchers(nil, TopicMatcher{Type: MatcherTypeExact, Pattern: "foo"}),
	}

	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.Header.Add("Authorization", bearerPrefix+signSubscriberToken(t, c))

	h := createDummy(t)

	claims, err := h.authorize(r, false)
	require.Error(t, err)
	require.Nil(t, claims)
}

// A token carrying a malformed authorization_details claim is rejected.
func TestAuthorizeRejectsInvalidAuthorizationDetails(t *testing.T) {
	t.Parallel()

	c := &claims{
		RegisteredClaims: subscriberRegisteredClaims(),
		AuthorizationDetails: []authorizationDetail{{
			Type:    authorizationDetailTypeMercure,
			Actions: []mercureAction{actionSubscribe},
			// No topics: invalid.
		}},
	}

	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.Header.Add("Authorization", bearerPrefix+signSubscriberToken(t, c))

	h := createDummy(t)

	claims, err := h.authorize(r, false)
	require.ErrorIs(t, err, errInvalidAuthorizationDetail)
	require.Nil(t, claims)
}

// A topic matcher carrying a control character is rejected, like the query and
// publish paths, so attacker-shaped patterns cannot reach the match cache.
func TestAuthorizeRejectsControlCharInAuthorizationDetail(t *testing.T) {
	t.Parallel()

	c := &claims{
		RegisteredClaims:     subscriberRegisteredClaims(),
		AuthorizationDetails: subscribeDetailsFromMatchers(nil, TopicMatcher{Type: MatcherTypeExact, Pattern: "foo\x00bar"}),
	}

	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.Header.Add("Authorization", bearerPrefix+signSubscriberToken(t, c))

	h := createDummy(t)

	claims, err := h.authorize(r, false)
	require.ErrorIs(t, err, errInvalidAuthorizationDetail)
	require.Nil(t, claims)
}

// When the hub advertises authorization servers, a token whose issuer is not
// one of them is rejected even if its signature, audience and expiration are
// valid (RFC 9068 §4).
func TestAuthorizeRejectsUntrustedIssuer(t *testing.T) {
	t.Parallel()

	rc := subscriberRegisteredClaims()
	rc.Issuer = "https://evil.example.com"
	c := &claims{
		RegisteredClaims:     rc,
		AuthorizationDetails: subscribeDetailsFromMatchers(nil, TopicMatcher{Type: MatcherTypeExact, Pattern: "foo"}),
	}

	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.Header.Add("Authorization", bearerPrefix+signSubscriberToken(t, c))

	h := createDummy(t, WithAuthorizationServers([]string{"https://auth.example.com"}))

	claims, err := h.authorize(r, false)
	require.ErrorIs(t, err, ErrInvalidJWT)
	require.Nil(t, claims)
}

// A token issued by one of the advertised authorization servers is accepted.
func TestAuthorizeAcceptsTrustedIssuer(t *testing.T) {
	t.Parallel()

	rc := subscriberRegisteredClaims()
	rc.Issuer = "https://auth.example.com"
	c := &claims{
		RegisteredClaims:     rc,
		AuthorizationDetails: subscribeDetailsFromMatchers(nil, TopicMatcher{Type: MatcherTypeExact, Pattern: "foo"}),
	}

	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.Header.Add("Authorization", bearerPrefix+signSubscriberToken(t, c))

	h := createDummy(t, WithAuthorizationServers([]string{"https://auth.example.com"}))

	claims, err := h.authorize(r, false)
	require.NoError(t, err)
	require.True(t, claims.authz.grants(h.topicMatcherStore, actionSubscribe, "foo"))
}
