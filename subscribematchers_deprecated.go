package mercure

import "errors"

// errTopicParamNotSupported is returned when the deprecated "topic" query
// parameter is used without WithProtocolVersionCompatibility.
var errTopicParamNotSupported = errors.New(`the "topic" query parameter is not supported anymore, use "match" instead`)

// appendDeprecatedTopicMatchers handles the v8 "topic" query parameter by
// wrapping each value in a deprecatedMatcher-backed topicMatcher. Only
// reachable when backward compatibility with protocol v8 is enabled; see
// parseMatchers.
func (h *Hub) appendDeprecatedTopicMatchers(matchers []topicMatcher, values []string) ([]topicMatcher, error) {
	for _, v := range values {
		if len(v) > maxPatternLength {
			return nil, errPatternTooLong
		}

		matchers = append(matchers, topicMatcher{
			Type:    deprecatedMatcherTypeName,
			Pattern: v,
			matcher: deprecatedMatcher,
		})

		if len(matchers) > maxMatcherCount {
			return nil, errTooManyMatchers
		}
	}

	return matchers, nil
}
