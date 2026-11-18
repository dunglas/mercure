package mercure

import (
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

var (
	selfKey     = []byte("self-key")     //nolint:gochecknoglobals
	explicitKey = []byte("explicit-key") //nolint:gochecknoglobals
)

func signSelfIssuerToken(tb testing.TB, key []byte, issuer, audience string) string {
	tb.Helper()

	token := jwt.New(jwt.SigningMethodHS256)
	token.Header["typ"] = atJWTType
	token.Claims = &claims{
		Issuer:               issuer,
		Audience:             jwt.ClaimStrings{audience},
		ExpiresAt:            jwt.NewNumericDate(time.Now().Add(time.Hour)),
		AuthorizationDetails: subscribeDetailsFromMatchers(nil, TopicMatcher{Type: MatcherTypeExact, Pattern: "foo"}),
	}

	s, err := token.SignedString(key)
	require.NoError(tb, err)

	return s
}

func newSelfIssuerHub(tb testing.TB, options ...Option) *Hub {
	tb.Helper()

	tms, err := NewTopicMatcherStore(0)
	require.NoError(tb, err)

	h, err := NewHub(tb.Context(), append([]Option{WithTopicMatcherStore(tms)}, options...)...)
	require.NoError(tb, err)

	return h
}

func authorizeOn(tb testing.TB, h *Hub, host, token string) error {
	tb.Helper()

	r, _ := http.NewRequestWithContext(NewRequestOriginContext(tb.Context(), schemeHTTPS, host), http.MethodGet, defaultHubURL, nil)
	r.Header.Add("Authorization", bearerPrefix+token)

	_, err := h.authorize(r, false)

	return err
}

func TestSelfIssuerTrustsRequestIdentity(t *testing.T) {
	t.Parallel()

	h := newSelfIssuerHub(t, WithIssuers([]Issuer{{Self: true, Subscriber: Static{Key: selfKey, Algorithm: "HS256"}}}))

	const (
		hubA = "https://a.example/.well-known/mercure"
		hubB = "https://b.example/.well-known/mercure"
	)

	for name, tc := range map[string]struct {
		host, issuer, audience string
		valid                  bool
	}{
		"origin":                {"a.example", "https://a.example", hubA, true},
		"hub URL":               {"a.example", hubA, hubA, true},
		"other host's identity": {"b.example", "https://b.example", hubB, true},
		"issuer of other host":  {"a.example", "https://b.example", hubA, false},
		"foreign issuer":        {"a.example", "https://evil.example", hubA, false},
		"origin with path":      {"a.example", "https://a.example/", hubA, false},
		"other scheme":          {"a.example", "http://a.example", hubA, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := authorizeOn(t, h, tc.host, signSelfIssuerToken(t, selfKey, tc.issuer, tc.audience))
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, ErrInvalidJWT)
			}
		})
	}
}

func TestSelfIssuerTrustsStaticResourceIdentifier(t *testing.T) {
	t.Parallel()

	h := newSelfIssuerHub(t,
		WithResourceIdentifier(testResourceIdentifier),
		WithIssuers([]Issuer{{Self: true, Subscriber: Static{Key: selfKey, Algorithm: "HS256"}}}),
	)

	for _, iss := range []string{testIssuer, testResourceIdentifier} {
		require.NoError(t, authorizeOn(t, h, "other.example", signSelfIssuerToken(t, selfKey, iss, testResourceIdentifier)), iss)
	}

	require.ErrorIs(t, authorizeOn(t, h, "other.example", signSelfIssuerToken(t, selfKey, "https://other.example", testResourceIdentifier)), ErrInvalidJWT)
}

// An explicit issuer disables the default: the hub URL is no longer trusted.
func TestExplicitIssuerDoesNotTrustHubURL(t *testing.T) {
	t.Parallel()

	h := newSelfIssuerHub(t, WithIssuers([]Issuer{{Identifier: "https://app.example", Subscriber: Static{Key: explicitKey, Algorithm: "HS256"}}}))

	const hub = "https://a.example/.well-known/mercure"

	require.NoError(t, authorizeOn(t, h, "a.example", signSelfIssuerToken(t, explicitKey, "https://app.example", hub)))
	require.ErrorIs(t, authorizeOn(t, h, "a.example", signSelfIssuerToken(t, explicitKey, "https://a.example", hub)), ErrInvalidJWT)
	require.ErrorIs(t, authorizeOn(t, h, "a.example", signSelfIssuerToken(t, explicitKey, hub, hub)), ErrInvalidJWT)
}

// Keys stay bound to their issuer when the self issuer sits next to explicit
// ones, and an explicit issuer naming the hub URL wins over the self issuer.
func TestSelfIssuerAlongsideExplicitIssuers(t *testing.T) {
	t.Parallel()

	const hub = "https://a.example/.well-known/mercure"

	h := newSelfIssuerHub(t, WithIssuers([]Issuer{
		{Self: true, Subscriber: Static{Key: selfKey, Algorithm: "HS256"}},
		{Identifier: "https://app.example", Subscriber: Static{Key: explicitKey, Algorithm: "HS256"}},
		{Identifier: hub, Subscriber: Static{Key: explicitKey, Algorithm: "HS256"}},
	}))

	for name, tc := range map[string]struct {
		key    []byte
		issuer string
		valid  bool
	}{
		"self, self key":            {selfKey, "https://a.example", true},
		"self, explicit key":        {explicitKey, "https://a.example", false},
		"explicit, explicit key":    {explicitKey, "https://app.example", true},
		"explicit, self key":        {selfKey, "https://app.example", false},
		"explicit hub URL wins":     {explicitKey, hub, true},
		"self key on explicit name": {selfKey, hub, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := authorizeOn(t, h, "a.example", signSelfIssuerToken(t, tc.key, tc.issuer, hub))
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, ErrInvalidJWT)
			}
		})
	}
}

func TestSelfIssuerConfigErrors(t *testing.T) {
	t.Parallel()

	v := Static{Key: selfKey, Algorithm: "HS256"}

	for name, tc := range map[string]struct {
		issuers []Issuer
		err     error
	}{
		"duplicate":            {[]Issuer{{Self: true, Subscriber: v}, {Self: true, Subscriber: v}}, ErrDuplicateIssuer},
		"identifier":           {[]Issuer{{Self: true, Identifier: testIssuer, Subscriber: v}}, ErrInvalidSelfIssuer},
		"authorization server": {[]Issuer{{Self: true, AuthorizationServer: true, Subscriber: v}}, ErrInvalidSelfIssuer},
		"no key":               {[]Issuer{{Self: true}}, ErrIssuerMissingKey},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := NewHub(t.Context(), WithIssuers(tc.issuers))
			require.ErrorIs(t, err, tc.err)
		})
	}
}
