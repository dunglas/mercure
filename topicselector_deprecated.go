//go:build deprecated_topic

package mercure

import "slices"

// matchDeprecatedMatcher implements the v8 matching rules: first an exact
// case-sensitive comparison, then a URI Template fallback. It powers both the
// v8 `topic=` query parameter and bare-string JWT claims when the hub is
// compiled with the deprecated_topic build tag.
func (tss *TopicSelectorStore) matchDeprecatedMatcher(topics []string, m topicMatcher) bool {
	if m.Type != deprecatedMatcherTypeName {
		return false
	}

	if slices.Contains(topics, m.Pattern) {
		return true
	}

	r := tss.getRegexp(m.Pattern)
	if r == nil {
		return false
	}

	return tss.cachedMatch(topics, m, func(ts []string, _ string) bool {
		return slices.ContainsFunc(ts, r.MatchString)
	})
}
