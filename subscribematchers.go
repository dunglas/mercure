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

	// paramTopic selects the Exact matcher type. Under
	// WithProtocolVersionCompatibility(8) and the deprecated_topic build tag
	// it keeps the v8 "exact or URI Template" semantics instead.
	paramTopic = "topic"
	// paramTopicURLPattern selects the URLPattern matcher type.
	paramTopicURLPattern = "topicURLPattern"

	// paramMatch and paramMatchType are the path-variable names mux extracts
	// for the subscription API URLs.
	paramMatch     = "match"
	paramMatchType = "matchType"
)

var (
	errMissingTopic = errors.New(`missing "topic" or "topicURLPattern" parameter`)
	// errUnknownMatcherParam is returned for query parameters that look like
	// topic matchers but are not defined by the protocol; parameter names are
	// case-sensitive and the request must be rejected per the spec.
	errUnknownMatcherParam = errors.New("unknown topic matcher query parameter")
	errInvalidMatcherValue = errors.New("topic matcher values must be valid UTF-8 without control characters")
	errTooManyMatchers     = fmt.Errorf("too many matchers (max %d)", maxMatcherCount)
	errPatternTooLong      = fmt.Errorf("pattern too long (max %d bytes)", maxPatternLength)
)

// parseMatchers extracts topic matchers from query parameters:
//   - `topic` → Exact matching (v8 semantics under compatibility mode, see
//     appendDeprecatedTopicMatchers in subscribematchers_deprecated.go)
//   - `topicURLPattern` → URL Pattern matching
//
// Any other parameter starting with "topic" (case-insensitively, to catch
// case typos of the case-sensitive names) is rejected with an error mapped
// to a 400 status code.
func (h *Hub) parseMatchers(query url.Values, deprecated bool) ([]topicMatcher, error) {
	var matchers []topicMatcher

	for key, values := range query {
		var matcherType MatcherType

		switch key {
		case paramTopic:
			if deprecated {
				m, err := h.appendDeprecatedTopicMatchers(matchers, values)
				if err != nil {
					return nil, err
				}

				matchers = m

				continue
			}

			matcherType = MatcherTypeExact
		case paramTopicURLPattern:
			matcherType = MatcherTypeURLPattern
		default:
			if len(key) >= len(paramTopic) && strings.EqualFold(key[:len(paramTopic)], paramTopic) {
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
		return nil, errMissingTopic
	}

	return matchers, nil
}

// appendMatchers validates each value of one topic matcher query parameter
// and appends one topicMatcher per value.
func (h *Hub) appendMatchers(matchers []topicMatcher, matcherType MatcherType, values []string) ([]topicMatcher, error) {
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

		m := topicMatcher{Type: matcherType, Pattern: v}
		if err := h.topicSelectorStore.validatePattern(m); err != nil {
			return nil, fmt.Errorf("invalid %s pattern: %w", matcherType, err)
		}

		matchers = append(matchers, m)
	}

	return matchers, nil
}
