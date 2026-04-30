package mercure

import (
	"fmt"
	"slices"
	"sync"

	urlpattern "github.com/dunglas/go-urlpattern"
)

// URLPatternMatcher is the built-in URL Pattern (WHATWG Living Standard) matching implementation.
var URLPatternMatcher Matcher = &urlPatternMatcherType{} //nolint:gochecknoglobals

// urlPatternBase is a synthetic base URL applied to both pattern compilation
// and topic testing. The protocol allows relative URL patterns and relative
// topics (e.g. "/.well-known/mercure/subscriptions/Exact/:topic/:subscriber"
// — the shape used by the hub's own subscription events) and mandates that
// the hub resolve them against the hub's URL. The library has no configured
// public URL, but matching is identity-preserving as long as both sides use
// the same base — so a fixed synthetic origin gives correct semantics without
// requiring deployment-specific configuration. ".invalid" is reserved by
// RFC 6761 §6.4, so it cannot collide with a real absolute pattern.
const urlPatternBase = "http://mercure.invalid"

type urlPatternMatcherType struct {
	patterns sync.Map
}

func (u *urlPatternMatcherType) Match(topics []string, pattern string) bool {
	p, err := u.getOrCompile(pattern)
	if err != nil {
		return false
	}

	return slices.ContainsFunc(topics, func(t string) bool { return p.Test(t, urlPatternBase) })
}

// Validate compiles the pattern up front and surfaces any parse error from
// the URL Pattern library. Both absolute and relative patterns are accepted;
// relative patterns are anchored at the hub's URL per the spec.
func (u *urlPatternMatcherType) Validate(pattern string) error {
	_, err := u.getOrCompile(pattern)

	return err
}

func (u *urlPatternMatcherType) getOrCompile(pattern string) (*urlpattern.URLPattern, error) {
	if cached, ok := u.patterns.Load(pattern); ok {
		return cached.(*urlpattern.URLPattern), nil
	}

	p, err := urlpattern.New(pattern, urlPatternBase, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid URL pattern: %w", err)
	}

	actual, _ := u.patterns.LoadOrStore(pattern, p)

	return actual.(*urlpattern.URLPattern), nil
}
