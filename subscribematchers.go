package mercure

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

const (
	maxMatcherCount  = 100
	maxPatternLength = 4096

	// paramMatch is both the subscribe query parameter selecting the Exact
	// matcher type and the mux path-variable name for the subscription API
	// URLs. A "match<MatcherType>" query parameter (e.g. "matchURLPattern")
	// selects that matcher type; bare "match" defaults to Exact, mirroring the
	// optional matchType of authorization details.
	paramMatch = "match"
	// paramMatchType is the mux path-variable name for the matcher type in the
	// subscription API URLs.
	paramMatchType = "matchType"

	// paramTopic is the deprecated v8 subscribe query parameter (exact or URI
	// Template), honored only under WithProtocolVersionCompatibility(8) and the
	// deprecated_topic build tag. It is also the path-variable name of the
	// deprecated /subscriptions/{topic} routes.
	paramTopic = "topic"
)

var (
	errMissingMatcher = errors.New(`missing "match" subscription parameter`)
	// errUnknownMatcherParam is returned for query parameters in the reserved
	// "match" namespace that are not defined by the protocol (an unknown
	// matcher type or a case typo); parameter names are case-sensitive and the
	// request must be rejected per the spec.
	errUnknownMatcherParam = errors.New("unknown topic matcher query parameter")
	errInvalidMatcherValue = errors.New("topic matcher values must be valid UTF-8 without control characters")
	errTooManyMatchers     = fmt.Errorf("too many matchers (max %d)", maxMatcherCount)
	errPatternTooLong      = fmt.Errorf("pattern too long (max %d bytes)", maxPatternLength)
	// errInvalidMatcherPattern wraps a pattern-compilation failure. The
	// underlying compiler error is kept in the chain for logging but must never
	// be written to the client: for some malformed URL Patterns go-urlpattern
	// returns a struct dump embedding a live heap pointer (CWE-209).
	errInvalidMatcherPattern = errors.New("invalid topic matcher pattern")
)

// parseMatchers extracts topic matchers from the subscribe query parameters:
//   - "match" → Exact matching (the default matcher type)
//   - "match<MatcherType>" (e.g. "matchURLPattern") → that matcher type
//   - "topic" → the deprecated v8 parameter (exact or URI Template), honored
//     only in compatibility mode; see appendDeprecatedTopicMatchers.
//
// Any other parameter in the reserved "match" namespace (an unknown matcher
// type or a case typo of a known one) is rejected with an error mapped to a
// 400 status code. Parameter names are case-sensitive.
func (h *Hub) parseMatchers(query url.Values, deprecated bool) ([]TopicMatcher, error) {
	var matchers []TopicMatcher

	for key, values := range query {
		if key == paramTopic {
			if !deprecated {
				return nil, fmt.Errorf("%w: %q (use %q or %q)", errUnknownMatcherParam, key, paramMatch, paramMatch+string(MatcherTypeURLPattern))
			}

			m, err := h.appendDeprecatedTopicMatchers(matchers, values)
			if err != nil {
				return nil, err
			}

			matchers = m

			continue
		}

		matcherType, ok := matcherTypeFromParam(key)
		if !ok {
			// Reject anything in the reserved "match" namespace that is not a
			// valid matcher parameter (an unknown matcher type or a case typo
			// of a known name) instead of silently ignoring it. The prefix
			// check is case-insensitive to catch typos of the case-sensitive
			// names.
			if len(key) >= len(paramMatch) && strings.EqualFold(key[:len(paramMatch)], paramMatch) {
				return nil, fmt.Errorf("%w: %q", errUnknownMatcherParam, key)
			}

			continue
		}

		m, err := h.appendMatchers(matchers, matcherType, values)
		if err != nil {
			return nil, err
		}

		matchers = m
	}

	if len(matchers) == 0 {
		return nil, errMissingMatcher
	}

	return matchers, nil
}

// matcherTypeFromParam maps a subscribe query parameter name to its matcher
// type. Bare "match" is the Exact default (mirroring the optional matchType of
// authorization details); "match<MatcherType>" selects that type. The boolean
// is false when the name is not in the "match" namespace or names an unknown
// matcher type.
func matcherTypeFromParam(key string) (MatcherType, bool) {
	suffix, ok := strings.CutPrefix(key, paramMatch)
	if !ok {
		return "", false
	}

	mt := MatcherType(suffix)
	if mt == "" {
		mt = MatcherTypeExact
	}

	switch mt {
	case MatcherTypeExact, MatcherTypeURLPattern:
		return mt, true
	case deprecatedMatcherTypeName:
		// The internal deprecated type is not addressable from the wire.
		return "", false
	default:
		return "", false
	}
}

// appendMatchers validates each value of one topic matcher query parameter
// and appends one TopicMatcher per value.
func (h *Hub) appendMatchers(matchers []TopicMatcher, matcherType MatcherType, values []string) ([]TopicMatcher, error) {
	for _, v := range values {
		if len(matchers) >= maxMatcherCount {
			return nil, errTooManyMatchers
		}

		if len(v) > maxPatternLength {
			return nil, errPatternTooLong
		}

		if !validProtocolString(v) {
			return nil, errInvalidMatcherValue
		}

		m := TopicMatcher{Type: matcherType, Pattern: v}
		if err := h.topicSelectorStore.validatePattern(m); err != nil {
			return nil, fmt.Errorf("%w (%s): %w", errInvalidMatcherPattern, matcherType, err)
		}

		matchers = append(matchers, m)
	}

	return matchers, nil
}
