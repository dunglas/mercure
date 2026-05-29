//go:build !deprecated_topic

package mercure

// appendDeprecatedTopicMatchers is the stub compiled without the
// deprecated_topic build tag: parseMatchers rejects the v8 "topic" query
// parameter regardless of WithProtocolVersionCompatibility because the
// deprecatedMatcher implementation is not in the binary.
func (h *Hub) appendDeprecatedTopicMatchers([]topicMatcher, []string) ([]topicMatcher, error) {
	return nil, errTopicParamNotSupported
}
