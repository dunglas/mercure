//go:build deprecated_topic

package mercure

import (
	"errors"
	"math"
	"regexp"
	"regexp/syntax"
	"slices"
	"strings"

	"github.com/yosida95/uritemplate/v3"
)

// matchDeprecated implements the v8 matching rules: first an exact
// case-sensitive comparison, then a URI Template fallback. It powers both the
// v8 `topic=` query parameter and bare-string JWT claims when the hub is
// compiled with the deprecated_topic build tag.
func (tms *TopicMatcherStore) matchDeprecated(topics []string, m TopicMatcher) bool {
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

// deprecatedMatcherTypeCompiled reports whether the v8 matcher-type code
// (exact-or-URI-Template semantics) is compiled into this binary. Used to
// decide whether a bare-string JWT claim can resolve to the deprecated
// matcher type, independent of whether alternate topics are granted.
func deprecatedMatcherTypeCompiled() bool {
	return true
}

var errURITemplateTooComplex = errors.New("URI template too complex to compile")

// validateDeprecated refuses a v8 URI template whose regexp cannot be compiled.
// Selectors that are not valid URI templates fall back to exact comparison.
func (tms *TopicMatcherStore) validateDeprecated(pattern string) error {
	if !strings.Contains(pattern, "{") {
		return nil
	}

	if _, err := uritemplate.New(pattern); err == nil && tms.getRegexp(pattern) == nil {
		return errURITemplateTooComplex
	}

	return nil
}

// getRegexp retrieves the regexp for this v8 template selector.
func (tms *TopicMatcherStore) getRegexp(pattern string) *regexp.Regexp {
	// If it's definitely not a URI template, skip to save some resources
	if !strings.Contains(pattern, "{") {
		return nil
	}

	if tms.templateCache != nil {
		if r, found := tms.templateCache.GetIfPresent(pattern); found {
			return r.value
		}
	}

	// If an error occurs, it's a raw string
	if tpl, err := uritemplate.New(pattern); err == nil {
		// Use template.Regexp() instead of template.Match() for performance
		// See https://github.com/yosida95/uritemplate/pull/7
		r := templateRegexp(tpl)

		if tms.templateCache != nil {
			cacheCompiled(tms.templateCache, tms.compiledCacheWeight, pattern, r, uint64(len(pattern))+templateWeight(r))
		}

		return r
	}

	return nil
}

func templateWeight(r *regexp.Regexp) uint64 {
	const overhead = 1 << 10

	if r == nil {
		return overhead
	}

	re, err := syntax.Parse(r.String(), syntax.Perl)
	if err != nil {
		return math.MaxUint32
	}

	return overhead + regexpWeight(re)
}

// templateRegexp recovers from uritemplate compiling a regexp past Go's repeat limit, which a subscriber-chosen selector can reach.
func templateRegexp(tpl *uritemplate.Template) (r *regexp.Regexp) {
	defer func() {
		if recover() != nil {
			r = nil
		}
	}()

	return tpl.Regexp()
}
