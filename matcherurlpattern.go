package mercure

import (
	"fmt"
	"slices"
	"sync"

	urlpattern "github.com/dunglas/go-urlpattern"
)

// urlPatternFallbackBase is the base URL applied when no public URL is
// configured. ".invalid" is reserved by RFC 6761 §6.4, so it cannot
// collide with a real absolute pattern. Relative ↔ relative matching is
// identity-preserving against any consistent base, so subscription
// events (which use relative topics) match correctly even without
// configuration. Cross-mode matching (a relative pattern against an
// absolute topic on the hub URL or vice versa) requires the real public
// URL — wire it through NewURLPatternMatcher / WithPublicURL.
const urlPatternFallbackBase = "http://mercure.invalid"

// NewURLPatternMatcher returns a URL Pattern (WHATWG Living Standard)
// matcher that resolves relative URL patterns and relative topics against
// baseURL, per the protocol's "the hub MUST use the hub's URL as the base
// URL" rule. Pass an empty string to fall back to a synthetic origin —
// only relative ↔ relative and absolute ↔ absolute matches will work in
// that case.
func NewURLPatternMatcher(baseURL string) Matcher { //nolint:ireturn
	if baseURL == "" {
		baseURL = urlPatternFallbackBase
	}

	return &urlPatternMatcherType{baseURL: baseURL}
}

type urlPatternMatcherType struct {
	baseURL  string
	patterns sync.Map
}

func (u *urlPatternMatcherType) Match(topics []string, pattern string) bool {
	p, err := u.getOrCompile(pattern)
	if err != nil {
		return false
	}

	return slices.ContainsFunc(topics, func(t string) bool { return p.Test(t, u.baseURL) })
}

// Validate compiles the pattern up front and surfaces any parse error from
// the URL Pattern library. Both absolute and relative patterns are accepted;
// relative patterns are anchored at the configured base URL per the spec.
func (u *urlPatternMatcherType) Validate(pattern string) error {
	_, err := u.getOrCompile(pattern)

	return err
}

func (u *urlPatternMatcherType) getOrCompile(pattern string) (*urlpattern.URLPattern, error) {
	if cached, ok := u.patterns.Load(pattern); ok {
		return cached.(*urlpattern.URLPattern), nil
	}

	p, err := urlpattern.New(pattern, u.baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid URL pattern: %w", err)
	}

	actual, _ := u.patterns.LoadOrStore(pattern, p)

	return actual.(*urlpattern.URLPattern), nil
}
