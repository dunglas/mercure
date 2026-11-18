//go:build deprecated_claim

package mercure

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

// In compatibility mode a sole self issuer still verifies 0.x tokens, which
// carry no meaningful iss, but 1.x tokens must name the hub.
func TestCompatModeSoleSelfIssuer(t *testing.T) {
	t.Parallel()

	const hub = "https://a.example/.well-known/mercure"

	h := newSelfIssuerHub(t,
		WithProtocolVersionCompatibility(8),
		WithIssuers([]Issuer{{Self: true, Subscriber: Static{Key: selfKey, Algorithm: "HS256"}}}),
	)

	legacy := jwt.NewWithClaims(jwt.SigningMethodHS256, &claims{
		Issuer:  "https://anything.example",
		Mercure: mercureClaim{Payload: "legacy"},
	})
	legacyToken, err := legacy.SignedString(selfKey)
	require.NoError(t, err)

	require.NoError(t, authorizeOn(t, h, "a.example", legacyToken))
	require.NoError(t, authorizeOn(t, h, "a.example", signSelfIssuerToken(t, selfKey, "https://a.example", hub)))
	require.ErrorIs(t, authorizeOn(t, h, "a.example", signSelfIssuerToken(t, selfKey, "https://anything.example", hub)), ErrInvalidJWT)
}
