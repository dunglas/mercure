package mercure

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

const (
	maxMatcherCount  = 100
	maxPatternLength = 4096
)

var (
	errMissingTopicOrMatch = errors.New(`missing "topic" or "match" parameter`)
	// errTopicParamNotSupported is returned when the deprecated "topic"
	// query parameter is used without WithProtocolVersionCompatibility, or
	// when the hub was compiled without the deprecated_topic build tag.
	errTopicParamNotSupported = errors.New(`the "topic" query parameter is not supported anymore, use "match" instead`)
	errTooManyMatchers        = fmt.Errorf("too many matchers (max %d)", maxMatcherCount)
	errPatternTooLong         = fmt.Errorf("pattern too long (max %d bytes)", maxPatternLength)
)

// parseMatchers extracts topic matchers from query parameters:
//   - `match` / `matchExact` → Exact matching
//   - `match{Type}` → the specified matcher type (case-insensitive)
//   - deprecated `topic` (only under WithProtocolVersionCompatibility), see
//     appendDeprecatedTopicMatchers in subscribematchers_deprecated.go.
func (h *Hub) parseMatchers(query url.Values, deprecated bool) ([]topicMatcher, error) {
	var matchers []topicMatcher

	for key, values := range query {
		keyLower := strings.ToLower(key)

		switch {
		case keyLower == "topic":
			if !deprecated {
				return nil, errTopicParamNotSupported
			}

			m, err := h.appendDeprecatedTopicMatchers(matchers, values)
			if err != nil {
				return nil, err
			}

			matchers = m

		case strings.HasPrefix(keyLower, "match"):
			m, err := h.appendMatchers(matchers, keyLower[5:], key, values)
			if err != nil {
				return nil, err
			}

			matchers = m
		}
	}

	if len(matchers) == 0 {
		return nil, errMissingTopicOrMatch
	}

	return matchers, nil
}

// appendMatchers resolves one `match*` query parameter to a registered
// matcher and appends one topicMatcher per value.
func (h *Hub) appendMatchers(matchers []topicMatcher, suffix, originalKey string, values []string) ([]topicMatcher, error) {
	typeName := suffix
	if typeName == "" {
		typeName = exactMatcherTypeName
	}

	rm, ok := h.topicSelectorStore.matchers[typeName]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedMatcherType, originalKey)
	}

	for _, v := range values {
		if len(matchers) >= maxMatcherCount {
			return nil, errTooManyMatchers
		}

		if len(v) > maxPatternLength {
			return nil, errPatternTooLong
		}

		if err := validatePattern(rm.matcher, v); err != nil {
			return nil, fmt.Errorf("invalid %s pattern: %w", rm.canonicalName, err)
		}

		matchers = append(matchers, topicMatcher{
			Type:    rm.canonicalName,
			Pattern: v,
			matcher: rm.matcher,
		})
	}

	return matchers, nil
}
