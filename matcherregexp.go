package mercure

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"
)

var (
	errIRegexpAnchors        = errors.New("I-Regexp must not contain anchors (^ or $)")
	errIRegexpBackreferences = errors.New("I-Regexp must not contain backreferences")
	errIRegexpInvalidUTF8    = errors.New("I-Regexp must be valid UTF-8")
)

// RegexpMatcher is the built-in I-Regexp (RFC 9485) matching implementation.
var RegexpMatcher Matcher = &regexpMatcherType{} //nolint:gochecknoglobals

type regexpMatcherType struct {
	patterns sync.Map
}

func (r *regexpMatcherType) Match(topics []string, pattern string) bool {
	re, err := r.getOrCompile(pattern)
	if err != nil {
		return false
	}

	return slices.ContainsFunc(topics, re.MatchString)
}

func (r *regexpMatcherType) getOrCompile(pattern string) (*regexp.Regexp, error) {
	if cached, ok := r.patterns.Load(pattern); ok {
		return cached.(*regexp.Regexp), nil
	}

	re, err := compileIRegexp(pattern)
	if err != nil {
		return nil, err
	}

	actual, _ := r.patterns.LoadOrStore(pattern, re)

	return actual.(*regexp.Regexp), nil
}

// compileIRegexp validates an I-Regexp pattern (RFC 9485) and compiles it to a Go regexp.
// I-Regexp is a subset of XSD regex designed for interoperability.
// Key constraints: no backreferences, no lookahead/lookbehind, no anchors (^ and $).
// The pattern implicitly matches the entire string (anchored).
func compileIRegexp(pattern string) (*regexp.Regexp, error) {
	if err := validateIRegexp(pattern); err != nil {
		return nil, err
	}

	re, err := regexp.Compile("^(?:" + pattern + ")$")
	if err != nil {
		return nil, fmt.Errorf("invalid I-Regexp pattern: %w", err)
	}

	return re, nil
}

// validateIRegexp validates that a pattern conforms to I-Regexp (RFC 9485).
func validateIRegexp(pattern string) error {
	if strings.HasPrefix(pattern, "^") || strings.HasSuffix(pattern, "$") {
		return fmt.Errorf("%w: %q", errIRegexpAnchors, pattern)
	}

	for i := 0; i < len(pattern)-1; i++ {
		if pattern[i] == '\\' {
			next := pattern[i+1]
			if next >= '1' && next <= '9' {
				return fmt.Errorf("%w: %q", errIRegexpBackreferences, pattern)
			}

			i++
		}
	}

	if !utf8.ValidString(pattern) {
		return fmt.Errorf("%w: %q", errIRegexpInvalidUTF8, pattern)
	}

	return nil
}
