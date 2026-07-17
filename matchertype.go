package mercure

import (
	"errors"
	"unicode"
	"unicode/utf8"
)

// MatcherType identifies how a topic matcher pattern is evaluated.
// The constant values are canonical: they double as the wire format in
// subscription API URLs, subscription event payloads and JWT claims.
type MatcherType string

const (
	// MatcherTypeExact selects byte-for-byte topic comparison. The reserved
	// pattern "*" matches every topic.
	MatcherTypeExact MatcherType = "exact"

	// MatcherTypeURLPattern selects matching per the WHATWG URL Pattern
	// Living Standard, with the hub's public URL as the base URL.
	MatcherTypeURLPattern MatcherType = "urlpattern"

	// deprecatedMatcherTypeName tags topic matchers created from the v8
	// `topic=` query parameter or bare-string JWT claims (exact-or-URI-Template
	// semantics). The underscore prefix keeps it out of the protocol namespace;
	// it is only honored in builds with the deprecated_topic tag.
	deprecatedMatcherTypeName MatcherType = "_deprecated_topic"
)

// ErrUnsupportedMatcherType is returned when a matcher type is not one of the
// types defined by the protocol. The HTTP handlers map it to a 400 status code.
var ErrUnsupportedMatcherType = errors.New("unsupported topic matcher type")

// TopicMatcher pairs a matcher type with a pattern string. It is the exported
// value type transports use to (re)construct a Subscriber's matchers via
// Subscriber.SetMatchers.
type TopicMatcher struct {
	Type    MatcherType
	Pattern string
}

// validProtocolString reports whether s satisfies the constraints the protocol
// puts on topics, matcher patterns and the id/type fields: valid UTF-8 without
// control characters — C0 (U+0000–U+001F), DEL (U+007F), or C1
// (U+0080–U+009F) — and without Unicode format characters (category Cf:
// bidirectional and zero-width controls), which are invisible and enable
// identifier spoofing (Trojan Source). NUL rejection also protects the match
// cache, which joins topics with a NUL separator.
func validProtocolString(s string) bool {
	// Single pass: DecodeRuneInString flags invalid UTF-8 (RuneError with a
	// one-byte width) so a separate utf8.ValidString walk is unnecessary.
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			return false
		}

		if r <= 0x1F || (r >= 0x7F && r <= 0x9F) || unicode.Is(unicode.Cf, r) {
			return false
		}

		i += size
	}

	return true
}

// knownMatcherType reports whether mt is a matcher type addressable from the
// wire (a token, a subscribe query parameter, or a subscription API URL). The
// internal deprecated type is excluded. This is the single definition of the
// protocol's matcher-type set; a new type is added here and to the per-type
// dispatch in TopicMatcherStore.validatePattern and matchMatcher.
func knownMatcherType(mt MatcherType) bool {
	switch mt {
	case MatcherTypeExact, MatcherTypeURLPattern:
		return true
	case deprecatedMatcherTypeName:
		// The internal deprecated type is not addressable from the wire.
		return false
	default:
		return false
	}
}
