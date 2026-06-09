package mercure

import (
	"errors"
	"unicode/utf8"
)

// MatcherType identifies how a topic matcher pattern is evaluated.
// The constant values are canonical: they double as the wire format in
// subscription API URLs, subscription event payloads and JWT claims.
type MatcherType string

const (
	// MatcherTypeExact selects byte-for-byte topic comparison. The reserved
	// pattern "*" matches every topic.
	MatcherTypeExact MatcherType = "Exact"

	// MatcherTypeURLPattern selects matching per the WHATWG URL Pattern
	// Living Standard, with the hub's public URL as the base URL.
	MatcherTypeURLPattern MatcherType = "URLPattern"

	// deprecatedMatcherTypeName tags topic matchers created from the v8
	// `topic=` query parameter or bare-string JWT claims (exact-or-URI-Template
	// semantics). The underscore prefix keeps it out of the protocol namespace;
	// it is only honored in builds with the deprecated_topic tag.
	deprecatedMatcherTypeName MatcherType = "_deprecated_topic"
)

// ErrUnsupportedMatcherType is returned when a matcher type is not one of the
// types defined by the protocol. The HTTP handlers map it to a 400 status code.
var ErrUnsupportedMatcherType = errors.New("unsupported topic matcher type")

// topicMatcher pairs a matcher type with a pattern string.
type topicMatcher struct {
	Type    MatcherType
	Pattern string
}

// validProtocolString reports whether s satisfies the constraints the protocol
// puts on topics and matcher patterns: valid UTF-8 without control characters —
// C0 (U+0000–U+001F), DEL (U+007F), or C1 (U+0080–U+009F). NUL rejection also
// protects the match cache, which joins topics with a NUL separator.
func validProtocolString(s string) bool {
	for _, r := range s {
		if r <= 0x1F || (r >= 0x7F && r <= 0x9F) {
			return false
		}
	}

	return utf8.ValidString(s)
}
