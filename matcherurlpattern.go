package mercure

import (
	"fmt"
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

	for _, topic := range topics {
		if p.Test(topic, "") {
			return true
		}
	}

	return false
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
