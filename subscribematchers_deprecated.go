//go:build deprecated_topic

package mercure

// appendDeprecatedTopicMatchers handles the v8 "topic" query parameter by
// wrapping each value in a deprecatedMatcher-backed topicMatcher. Reachable
// from parseMatchers only when WithProtocolVersionCompatibility(8) is set
// AND the hub was compiled with the deprecated_topic build tag.
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
