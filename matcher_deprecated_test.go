package mercure

// Test helpers that wrap v8-style string topic selectors into deprecatedMatcher-
// backed topicMatchers and matcherClaims. Kept in a _test.go file because
// they are only exercised by tests that pin the deprecated compatibility path.

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
