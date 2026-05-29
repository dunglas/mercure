//go:build deprecated_topic

package mercure

import (
	"slices"
	"strings"
)

// deprecatedMatcher implements the v8 matching rules: first an exact
// case-sensitive comparison, then a URI Template fallback. It powers both the
// v8 `topic=` query parameter and bare-string JWT claims when the hub is
// compiled with the deprecated_topic build tag.
//
// It is never registered in the public matcher registry: query parameters
// can't reach it by name, and the deprecated paths plug it in directly.
var deprecatedMatcher Matcher = deprecatedMatcherType{} //nolint:gochecknoglobals

type deprecatedMatcherType struct{}

func (deprecatedMatcherType) Match(topics []string, pattern string) bool {
	if slices.Contains(topics, pattern) {
		return true
	}

	// Shortcut: only fall through to URI template matching when the pattern
	// actually looks like one.
	if !strings.Contains(pattern, "{") {
		return false
	}

	return URITemplateMatcher.Match(topics, pattern)
}

// resolveDeprecatedStringClaim binds a bare-string claim to the
// deprecatedMatcher. Called by resolveMatcherClaims when the hub operates in
// backward-compatibility mode.
func resolveDeprecatedStringClaim(c *matcherClaim) error {
	c.Type = deprecatedMatcherTypeName
	c.matcher = deprecatedMatcher

	return nil
}
