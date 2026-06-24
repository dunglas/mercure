//go:build !deprecated_topic

package mercure

import "errors"

// errTopicParamCompatNotSupported is returned when the hub runs with
// WithProtocolVersionCompatibility but was compiled without the
// deprecated_topic build tag: the v8 matching code is not in the binary.
var errTopicParamCompatNotSupported = errors.New(`v8 "topic" semantics require a hub built with the deprecated_topic tag`)

// appendDeprecatedTopicMatchers is the stub compiled without the
// deprecated_topic build tag.
func (h *Hub) appendDeprecatedTopicMatchers([]topicMatcher, []string) ([]topicMatcher, error) {
	return nil, errTopicParamCompatNotSupported
}
