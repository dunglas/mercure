//go:build deprecated_topic && deprecated_claim

package mercure

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func stringsToDeprecatedClaims(patterns []string) []matcherClaim {
	matchers := stringsToDeprecatedMatchers(patterns)

	claims := make([]matcherClaim, len(matchers))
	for i, m := range matchers {
		claims[i] = matcherClaim{topicMatcher: m}
	}

	return claims
}

// createDeprecatedDummy builds a Hub exactly like createDummy, plus
// WithProtocolVersionCompatibility(8) so the test runs through the v8
// compat code path (bare-string JWT claims, `topic=` query parameter,
// deprecated subscription routes).
func createDeprecatedDummy(tb testing.TB, options ...Option) *Hub {
	tb.Helper()

	return createDummy(tb, append([]Option{WithProtocolVersionCompatibility(8)}, options...)...)
}

// createDeprecatedAuthorizedJWT mirrors createDummyAuthorizedJWT but emits
// v8 bare-string claims so topic selectors with `{var}` placeholders fall
// back to the URI-template matcher on the hub side.
//
//nolint:unparam // kept symmetric with createDummyAuthorizedJWT; role may be either.
func createDeprecatedAuthorizedJWT(r role, topics []string, payload ...any) string {
	var p any = struct {
		Foo string `json:"foo"`
	}{Foo: "bar"}
	if len(payload) > 0 {
		p = payload[0]
	}

	token := jwt.New(jwt.SigningMethodHS256)

	var key []byte

	switch r {
	case rolePublisher:
		token.Claims = &claims{
			deprecatedMercureClaims: deprecatedMercureClaims{Mercure: mercureClaim{Publish: stringsToDeprecatedClaims(topics)}},
			RegisteredClaims:        jwt.RegisteredClaims{},
		}
		key = []byte("publisher")

	case roleSubscriber:
		token.Claims = &claims{
			deprecatedMercureClaims: deprecatedMercureClaims{Mercure: mercureClaim{
				Subscribe: stringsToDeprecatedClaims(topics),
				Payload:   p,
			}},
			RegisteredClaims: jwt.RegisteredClaims{},
		}

		key = []byte("subscriber")
	}

	tokenString, _ := token.SignedString(key)

	return tokenString
}
