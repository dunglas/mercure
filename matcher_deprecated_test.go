//go:build deprecated_topic

package mercure

// Test helpers that wrap v8-style string topic selectors into deprecatedMatcher-
// backed topicMatchers and matcherClaims. Compiled only with the
// deprecated_topic build tag because deprecatedMatcher itself is gated.

func stringsToDeprecatedMatchers(patterns []string) []topicMatcher {
	if patterns == nil {
		return nil
	}

	out := make([]topicMatcher, len(patterns))
	for i, p := range patterns {
		out[i] = topicMatcher{Type: deprecatedMatcherTypeName, Pattern: p, matcher: deprecatedMatcher}
	}

	return out
}

func stringsToDeprecatedClaims(patterns []string) []matcherClaim {
	matchers := stringsToDeprecatedMatchers(patterns)

	claims := make([]matcherClaim, len(matchers))
	for i, m := range matchers {
		claims[i] = matcherClaim{topicMatcher: m}
	}

	return claims
}
