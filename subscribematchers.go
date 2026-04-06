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
	// errMissingTopicOrMatch is returned when no topic or match parameters are found.
	errMissingTopicOrMatch = errors.New(`missing "topic" or "match" parameter`)

	// errTopicParamNotSupported is returned when the legacy "topic" query parameter is used without backward compat.
	errTopicParamNotSupported = errors.New(`the "topic" query parameter is not supported anymore, use "match" instead`)

	// errTooManyMatchers is returned when the number of matchers exceeds the limit.
	errTooManyMatchers = fmt.Errorf("too many matchers (max %d)", maxMatcherCount)

	// errPatternTooLong is returned when a pattern exceeds the maximum length.
	errPatternTooLong = fmt.Errorf("pattern too long (max %d bytes)", maxPatternLength)
)

// parseMatchers extracts topic matchers from query parameters.
// It handles:
//   - The legacy `topic` parameter (case-insensitive, only when legacy=true)
//   - `match` / `matchExact` → Exact matching
//   - `match{Type}` → the specified matcher type (case-insensitive)
//
// Returns ErrUnsupportedMatcherType for unknown matcher types (→ 501).
func (h *Hub) parseMatchers(query url.Values, legacy bool) ([]topicMatcher, error) {
	var matchers []topicMatcher

	for key, values := range query {
		keyLower := strings.ToLower(key)

		switch {
		case keyLower == "topic":
			if !legacy {
				return nil, errTopicParamNotSupported
			}

			m, err := h.appendLegacyTopicMatchers(matchers, values)
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

func (h *Hub) appendLegacyTopicMatchers(matchers []topicMatcher, values []string) ([]topicMatcher, error) {
	for _, v := range values {
		if len(v) > maxPatternLength {
			return nil, errPatternTooLong
		}

		matchers = append(matchers, topicMatcher{
			Type:    legacyMatcherTypeName,
			Pattern: v,
			matcher: legacyMatcher,
		})

		if len(matchers) > maxMatcherCount {
			return nil, errTooManyMatchers
		}
	}

	return matchers, nil
}

func (h *Hub) appendMatchers(matchers []topicMatcher, suffix, originalKey string, values []string) ([]topicMatcher, error) {
	var typeName string

	switch suffix {
	case "", "exact":
		typeName = exactMatcherTypeName
	default:
		if _, ok := h.topicSelectorStore.ResolveMatcherType(suffix); !ok {
			return nil, fmt.Errorf("%w: %s", ErrUnsupportedMatcherType, originalKey)
		}

		typeName = suffix // already lowercase
	}

	for _, v := range values {
		if len(v) > maxPatternLength {
			return nil, errPatternTooLong
		}

		m, err := h.topicSelectorStore.newTopicMatcher(typeName, v)
		if err != nil {
			return nil, err
		}

		matchers = append(matchers, m)

		if len(matchers) > maxMatcherCount {
			return nil, errTooManyMatchers
		}
	}

	return matchers, nil
}
