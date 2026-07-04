//go:build !deprecated_topic

package mercure

// matchDeprecatedMatcher is the stub compiled without the deprecated_topic
// build tag: v8 matchers are not in the binary, so nothing matches.
func (tss *TopicSelectorStore) matchDeprecatedMatcher([]string, TopicMatcher) bool {
	return false
}
