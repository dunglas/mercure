package caddy

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// grantedActions parses the playground token and returns the set of granted actions.
func grantedActions(t *testing.T, signed, key string) (jwt.MapClaims, map[string]bool) {
	t.Helper()

	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(signed, claims, func(*jwt.Token) (any, error) { return []byte(key), nil })
	require.NoError(t, err)
	require.True(t, parsed.Valid)
	assert.Equal(t, accessTokenType, parsed.Header["typ"])

	actions := map[string]bool{}

	for _, d := range claims[claimAuthorizationDetails].([]any) {
		for _, a := range d.(map[string]any)["actions"].([]any) {
			actions[a.(string)] = true
		}
	}

	return claims, actions
}

func TestPlaygroundTokenFuncMintsAllAccessToken(t *testing.T) {
	t.Parallel()

	const key = "!ChangeMe!"

	m := &Mercure{Issuers: []IssuerConfig{{
		Identifier: "https://localhost",
		Publisher:  VerifierConfig{JWT: JWTConfig{Key: key}},
		Subscriber: VerifierConfig{JWT: JWTConfig{Key: key}},
	}}}

	fn := m.playgroundTokenFunc()
	require.NotNil(t, fn)

	signed, err := fn("https://hub.example.com/.well-known/mercure")
	require.NoError(t, err)

	claims, actions := grantedActions(t, signed, key)
	assert.Equal(t, "https://localhost", claims[claimIss])
	assert.Equal(t, "https://hub.example.com/.well-known/mercure", claims[claimAud])
	// A single key verifies both roles, so the token carries both grants.
	assert.True(t, actions["publish"])
	assert.True(t, actions["subscribe"])
}

func TestPlaygroundTokenFuncSubscribeOnlyWhenKeysDiffer(t *testing.T) {
	t.Parallel()

	m := &Mercure{Issuers: []IssuerConfig{{
		Identifier: "https://localhost",
		Publisher:  VerifierConfig{JWT: JWTConfig{Key: "publisher-secret"}},
		Subscriber: VerifierConfig{JWT: JWTConfig{Key: "subscriber-secret"}},
	}}}

	fn := m.playgroundTokenFunc()
	require.NotNil(t, fn)

	signed, err := fn("https://hub.example.com/.well-known/mercure")
	require.NoError(t, err)

	// The publisher key can't verify a token the subscriber key signed, so
	// granting publish would hand out an unusable claim: subscribe only.
	_, actions := grantedActions(t, signed, "subscriber-secret")
	assert.True(t, actions["subscribe"])
	assert.False(t, actions["publish"])
}

func TestPlaygroundTokenFuncNilWithoutSymmetricKey(t *testing.T) {
	t.Parallel()

	// A JWK Set is remote; the hub holds no signing key for it.
	jwks := &Mercure{Issuers: []IssuerConfig{{
		Identifier: "https://localhost",
		Subscriber: VerifierConfig{JWKSURL: "https://example.com/jwks.json"},
	}}}
	assert.Nil(t, jwks.playgroundTokenFunc())

	// A PEM key is asymmetric: only the public half is configured here.
	pem := &Mercure{Issuers: []IssuerConfig{{
		Identifier: "https://localhost",
		Subscriber: VerifierConfig{JWT: JWTConfig{Key: "-----BEGIN PUBLIC KEY-----\nZm9v\n-----END PUBLIC KEY-----", Alg: "RS256"}},
	}}}
	assert.Nil(t, pem.playgroundTokenFunc())

	// No verifier at all.
	assert.Nil(t, (&Mercure{}).playgroundTokenFunc())
}
