package mercure

import "slices"

// ExactMatcher is the built-in exact matching implementation.
// It performs a case-sensitive string comparison.
var ExactMatcher Matcher = exactMatcherType{} //nolint:gochecknoglobals

type exactMatcherType struct{}

func (exactMatcherType) Match(topics []string, pattern string) bool {
	return slices.Contains(topics, pattern)
}
