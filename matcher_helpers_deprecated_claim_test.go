//go:build deprecated_claim

package mercure

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

// Legacy mercure-claim test helpers, compiled only under the deprecated_claim
// build tag.

// createLegacyDummy builds a hub in compatibility mode, where the legacy
// mercure claim and the deprecated "authorization" query parameter are honored.
func createLegacyDummy(tb testing.TB, options ...Option) *Hub {
	tb.Helper()

	return createDummy(tb, append(options, WithProtocolVersionCompatibility(8))...)
}

func stringsToExactClaims(patterns []string) []matcherClaim {
	matchers := stringsToExactMatchers(patterns)

	claims := make([]matcherClaim, len(matchers))
	for i, m := range matchers {
		claims[i] = matcherClaim{topicMatcher: m}
	}

	return claims
}

func matcherClaimPatterns(claims []matcherClaim) []string {
	if claims == nil {
		return nil
	}

	patterns := make([]string, len(claims))
	for i, c := range claims {
		patterns[i] = c.Pattern
	}

	return patterns
}

// createDummySubscriberJWTWithClaims mints a legacy mercure-claim subscriber
// token (no typ/aud), accepted only in compatibility mode.
func createDummySubscriberJWTWithClaims(tb testing.TB, subscribe []matcherClaim, payload any) string {
	tb.Helper()

	token := jwt.New(jwt.SigningMethodHS256)
	token.Claims = &claims{
		deprecatedMercureClaims: deprecatedMercureClaims{Mercure: mercureClaim{Subscribe: subscribe, Payload: payload}},
		RegisteredClaims:        jwt.RegisteredClaims{},
	}

	tokenString, err := token.SignedString([]byte("subscriber"))
	require.NoError(tb, err)

	return tokenString
}
