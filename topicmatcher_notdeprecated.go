//go:build !deprecated_topic

package mercure

// matchDeprecated is the stub compiled without the deprecated_topic
// build tag: v8 matchers are not in the binary, so nothing matches.
func (tms *TopicMatcherStore) matchDeprecated([]string, TopicMatcher) bool {
	return false
}
