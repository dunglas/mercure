package mercure

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

// Test helpers that wrap a slice of topic strings into Exact-matcher
// topicMatchers and matcherClaims. Used by tests that don't specifically
// exercise the deprecated topic path.

func stringsToExactMatchers(patterns []string) []topicMatcher {
	if patterns == nil {
		return nil
	}

	out := make([]topicMatcher, len(patterns))
	for i, p := range patterns {
		out[i] = topicMatcher{Type: MatcherTypeExact, Pattern: p}
	}

	return out
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

func createDummySubscriberJWTWithClaims(tb testing.TB, subscribe []matcherClaim, payload any) string {
	tb.Helper()

	token := jwt.New(jwt.SigningMethodHS256)
	token.Claims = &claims{
		Mercure:          mercureClaim{Subscribe: subscribe, Payload: payload},
		RegisteredClaims: jwt.RegisteredClaims{},
	}

	tokenString, err := token.SignedString([]byte("subscriber"))
	require.NoError(tb, err)

	return tokenString
}
