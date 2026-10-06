//go:build deprecated_claim

package mercure

import (
	"testing"
)

// Legacy mercure-claim test helpers, compiled only under the deprecated_claim
// build tag.

// createLegacyDummy builds a hub in compatibility mode, where the legacy
// mercure claim and the deprecated "authorization" query parameter are honored.
func createLegacyDummy(tb testing.TB, options ...Option) *Hub {
	tb.Helper()

	return createDummy(tb, append(options, WithProtocolVersionCompatibility(8))...)
}

// legacyVerifier resets the hub to a single implicit issuer (empty identifier)
// with a static verifier for one role, reproducing the removed WithPublisherJWT
// and WithSubscriberJWT options for the compatibility-mode legacy tests.
func legacyVerifier(publish bool, key []byte, alg string) Option {
	return func(o *opt) error {
		kf, algs, err := Static{Key: key, Algorithm: alg}.buildKeyfunc()
		if err != nil {
			return err
		}

		var iv issuerVerifier
		if publish {
			iv.publisher = roleVerifier{keyfunc: kf, algorithms: algs}
		} else {
			iv.subscriber = roleVerifier{keyfunc: kf, algorithms: algs}
		}

		o.issuers = map[string]issuerVerifier{"": iv}
		o.publisherConfigured = iv.publisher.keyfunc != nil
		o.subscriberConfigured = iv.subscriber.keyfunc != nil

		return nil
	}
}

func withPublisherJWT(key []byte, alg string) Option  { return legacyVerifier(true, key, alg) }
func withSubscriberJWT(key []byte, alg string) Option { return legacyVerifier(false, key, alg) }

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
