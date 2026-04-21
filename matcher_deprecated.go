package mercure

import (
	"slices"
	"strings"
)

// deprecatedMatcher implements the v8 matching rules: first an exact case-sensitive
// comparison, then a URI Template fallback. It is used internally for:
//
//   - Legacy `topic` query parameter (v8 subscribe path, behind
//     WithProtocolVersionCompatibility).
//   - Plain string entries in the `mercure.subscribe` / `mercure.publish` JWT
//     claims in legacy mode — the modern spec interprets bare strings as
//     Exact, but v8 treats them as "exact OR URI Template".
//
// It is never registered in the public matcher registry: query parameters
// can't reach it by name, and the legacy paths plug it in directly.
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
