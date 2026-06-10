//go:build deprecated_topic

package mercure

// resolveDeprecatedStringClaim is not needed in deprecated builds: the
// resolution happens in resolveMatcherClaims directly. This file hosts the
// deprecated halves of the matcher parsing helpers instead.

// appendDeprecatedTopicMatchers wraps each value of the v8 `topic` query
// parameter into a deprecated topicMatcher (exact or URI Template matching).
// Only called when the hub runs under WithProtocolVersionCompatibility. It
// delegates to appendMatchers; validatePattern is a no-op for the deprecated
// type, which keeps the v8 "exact or URI Template" fallback.
func (h *Hub) appendDeprecatedTopicMatchers(matchers []topicMatcher, values []string) ([]topicMatcher, error) {
	return h.appendMatchers(matchers, deprecatedMatcherTypeName, values)
}
