//go:build deprecated_topic

package mercure

// resolveDeprecatedStringClaim is not needed in deprecated builds: the
// resolution happens in resolveMatcherClaims directly. This file hosts the
// deprecated halves of the matcher parsing helpers instead.

// appendDeprecatedTopicMatchers wraps each value of the v8 `topic` query
// parameter into a deprecated topicMatcher (exact or URI Template matching).
// Only called when the hub runs under WithProtocolVersionCompatibility.
func (h *Hub) appendDeprecatedTopicMatchers(matchers []topicMatcher, values []string) ([]topicMatcher, error) {
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

		matchers = append(matchers, topicMatcher{Type: deprecatedMatcherTypeName, Pattern: v})
	}

	return matchers, nil
}
