package mercure

import (
	"fmt"
	"slices"
	"sync"

	urlpattern "github.com/dunglas/go-urlpattern"
)

// URLPatternMatcher is the built-in URL Pattern (WHATWG Living Standard) matching implementation.
var URLPatternMatcher Matcher = &urlPatternMatcherType{} //nolint:gochecknoglobals

type urlPatternMatcherType struct {
	patterns sync.Map
}

func (u *urlPatternMatcherType) Match(topics []string, pattern string) bool {
	p, err := u.getOrCompile(pattern)
	if err != nil {
		return false
	}

	return slices.ContainsFunc(topics, func(t string) bool { return p.Test(t, "") })
}

// Validate compiles the pattern up front. It rejects relative URL patterns
// (the protocol requires absolute IRIs as topics, so a relative pattern would
// have no base URL to resolve against) and any other parse error the URL
// Pattern library reports.
func (u *urlPatternMatcherType) Validate(pattern string) error {
	_, err := u.getOrCompile(pattern)

	return err
}

func (u *urlPatternMatcherType) getOrCompile(pattern string) (*urlpattern.URLPattern, error) {
	if cached, ok := u.patterns.Load(pattern); ok {
		return cached.(*urlpattern.URLPattern), nil
	}

	p, err := urlpattern.New(pattern, "", nil)
	if err != nil {
		return nil, fmt.Errorf("invalid URL pattern: %w", err)
	}

	actual, _ := u.patterns.LoadOrStore(pattern, p)

	return actual.(*urlpattern.URLPattern), nil
}
