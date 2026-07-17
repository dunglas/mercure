//go:build deprecated_topic

package mercure

import (
	"regexp"
	"slices"
	"strings"

	"github.com/yosida95/uritemplate/v3"
)

// matchDeprecatedMatcher implements the v8 matching rules: first an exact
// case-sensitive comparison, then a URI Template fallback. It powers both the
// v8 `topic=` query parameter and bare-string JWT claims when the hub is
// compiled with the deprecated_topic build tag.
func (tms *TopicMatcherStore) matchDeprecatedMatcher(topics []string, m TopicMatcher) bool {
	if m.Type != deprecatedMatcherTypeName {
		return false
	}

	if slices.Contains(topics, m.Pattern) {
		return true
	}

	r := tms.getRegexp(m.Pattern)
	if r == nil {
		return false
	}

	return tms.cachedMatch(topics, m, func(ts []string, _ string) bool {
		return slices.ContainsFunc(ts, r.MatchString)
	})
}

// getRegexp retrieves the regexp for this v8 template selector.
func (tms *TopicMatcherStore) getRegexp(topicMatcher string) *regexp.Regexp {
	// If it's definitely not a URI template, skip to save some resources
	if !strings.Contains(topicMatcher, "{") {
		return nil
	}

	if tms.templateCache != nil {
		if r, found := tms.templateCache.GetIfPresent(topicMatcher); found {
			return r
		}
	}

	// If an error occurs, it's a raw string
	if tpl, err := uritemplate.New(topicMatcher); err == nil {
		// Use template.Regexp() instead of template.Match() for performance
		// See https://github.com/yosida95/uritemplate/pull/7
		r := tpl.Regexp()
		if tms.templateCache != nil {
			tms.templateCache.Set(topicMatcher, r)
		}

		return r
	}

	return nil
}
