package mercure

// Test helpers that wrap a slice of topic strings into Exact-matcher
// topicMatchers and matcherClaims. Used by tests that don't specifically
// exercise the deprecated topic path.

func stringsToExactMatchers(patterns []string) []topicMatcher {
	if patterns == nil {
		return nil
	}

	out := make([]topicMatcher, len(patterns))
	for i, p := range patterns {
		out[i] = topicMatcher{Type: "Exact", Pattern: p, matcher: ExactMatcher}
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
