package mercure

import (
	"regexp"
	"slices"
	"sync"

	"github.com/yosida95/uritemplate/v3"
)

// URITemplateMatcher is the built-in URI Template (RFC 6570) matching implementation.
var URITemplateMatcher Matcher = &uriTemplateMatcherType{} //nolint:gochecknoglobals

type uriTemplateMatcherType struct {
	patterns sync.Map
}

func (u *uriTemplateMatcherType) Match(topics []string, pattern string) bool {
	compiled, err := u.getOrCompile(pattern)
	if err != nil {
		return false
	}

	return slices.ContainsFunc(topics, compiled.MatchString)
}

func (u *uriTemplateMatcherType) getOrCompile(pattern string) (*regexp.Regexp, error) {
	if cached, ok := u.patterns.Load(pattern); ok {
		return cached.(*regexp.Regexp), nil
	}

	re, err := compileURITemplate(pattern)
	if err != nil {
		return nil, err
	}

	actual, _ := u.patterns.LoadOrStore(pattern, re)

	return actual.(*regexp.Regexp), nil
}

func compileURITemplate(pattern string) (*regexp.Regexp, error) {
	tpl, err := uritemplate.New(pattern)
	if err != nil {
		return nil, err //nolint:wrapcheck
	}

	return tpl.Regexp(), nil
}
