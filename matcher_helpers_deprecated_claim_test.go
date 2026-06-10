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
