package mercure

import (
	"strings"

	wurl "github.com/nlnwa/whatwg-url/url"
)

// addressesReservedNamespace uses the matching base to prevent subscription-event forgery.
func addressesReservedNamespace(topic string, base *wurl.Url) bool {
	if deprecatedMatcherTypeCompiled() && rawPathAddressesReservedNamespace(topic) {
		return true
	}

	resolved, err := base.Parse(topic)

	return err == nil && pathAddressesReservedNamespace(resolved)
}

// schemePrefixLen returns the length of the topic's leading "scheme:", or 0 when
// it has none. A scheme is only a scheme at the very start, so callers must not
// look for one with a substring search: a "://" further along belongs to the path.
func schemePrefixLen(topic string) int {
	for i := range len(topic) {
		switch c := topic[i]; {
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z':
		case i > 0 && ('0' <= c && c <= '9' || c == '+' || c == '-' || c == '.'):
		case i > 0 && c == ':':
			return i + 1
		default:
			return 0
		}
	}

	return 0
}

func pathAddressesReservedNamespace(resolved *wurl.Url) bool {
	path := decodeUnreservedPercentEncoding(resolved.Pathname())

	return path == defaultHubURL || strings.HasPrefix(path, defaultHubURL+"/")
}

// rawPathAddressesReservedNamespace reports whether the topic's path lies in the
// reserved namespace before dot segments are removed: the deprecated v8 matcher
// compares raw strings, so ".../subscriptions/../.." still matches a reserved
// pattern once the guard has normalized it out. Only called in a build that has
// that matcher (see addressesReservedNamespace).
func rawPathAddressesReservedNamespace(topic string) bool {
	path := topic

	if n := schemePrefixLen(path); n > 0 {
		path = path[n:]

		if authority, ok := strings.CutPrefix(path, "//"); ok {
			j := strings.IndexByte(authority, '/')
			if j == -1 {
				return false
			}

			path = authority[j:]
		}
	}

	path, _, _ = strings.Cut(path, "?")
	path, _, _ = strings.Cut(path, "#")
	path = decodeUnreservedPercentEncoding(path)

	return path == defaultHubURL || strings.HasPrefix(path, defaultHubURL+"/")
}

// decodeUnreservedPercentEncoding decodes %XX octets corresponding to RFC 3986
// unreserved characters (ALPHA / DIGIT / "-" / "." / "_" / "~"): the
// percent-encoding normalization of RFC 3986 §6.2.2.2.
func decodeUnreservedPercentEncoding(s string) string {
	i := strings.IndexByte(s, '%')
	if i == -1 {
		return s
	}

	var b strings.Builder

	b.Grow(len(s))
	b.WriteString(s[:i])

	for ; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			hi, okHi := unhex(s[i+1])
			lo, okLo := unhex(s[i+2])

			if okHi && okLo {
				if c := hi<<4 | lo; isUnreservedByte(c) {
					b.WriteByte(c)

					i += 2

					continue
				}
			}
		}

		b.WriteByte(s[i])
	}

	return b.String()
}

func unhex(c byte) (byte, bool) {
	switch {
	case '0' <= c && c <= '9':
		return c - '0', true
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10, true
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10, true
	default:
		return 0, false
	}
}

func isUnreservedByte(c byte) bool {
	return 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' ||
		'0' <= c && c <= '9' || c == '-' || c == '.' || c == '_' || c == '~'
}
