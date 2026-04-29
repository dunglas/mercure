package mercure

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMatchersExact(t *testing.T) {
	t.Parallel()

	h := createDummy(t, withAllMatcherTypes()...)

	query := url.Values{"match": {"foo", "bar"}}
	matchers, err := h.parseMatchers(query, false)
	require.NoError(t, err)

	assert.Len(t, matchers, 2)
	assert.Equal(t, "Exact", matchers[0].Type)
	assert.Equal(t, "foo", matchers[0].Pattern)
	assert.Equal(t, "Exact", matchers[1].Type)
	assert.Equal(t, "bar", matchers[1].Pattern)
}

func TestParseMatchersExactExplicit(t *testing.T) {
	t.Parallel()

	h := createDummy(t, withAllMatcherTypes()...)

	query := url.Values{"matchExact": {"foo"}}
	matchers, err := h.parseMatchers(query, false)
	require.NoError(t, err)

	assert.Len(t, matchers, 1)
	assert.Equal(t, "Exact", matchers[0].Type)
}

func TestParseMatchersCaseInsensitive(t *testing.T) {
	t.Parallel()

	h := createDummy(t, withAllMatcherTypes()...)

	// Query parameter names are case-insensitive for match* params
	query := url.Values{
		"MATCH":            {"a"},
		"MatchExact":       {"b"},
		"MATCHURLPATTERN":  {"https://example.com/:id"},
		"matchRegexp":      {".*"},
		"matchURITEMPLATE": {"https://example.com/{id}"},
	}

	matchers, err := h.parseMatchers(query, false)
	require.NoError(t, err)

	assert.Len(t, matchers, 5)

	types := make(map[string]bool)
	for _, m := range matchers {
		types[m.Type] = true
	}

	assert.True(t, types["Exact"])
	assert.True(t, types["URLPattern"])
	assert.True(t, types["Regexp"])
	assert.True(t, types["URITemplate"])
}

func TestParseMatchersTopicRejectedWhenNotDeprecated(t *testing.T) {
	t.Parallel()

	h := createDummy(t, withAllMatcherTypes()...)

	query := url.Values{"topic": {"foo"}}
	_, err := h.parseMatchers(query, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not supported anymore")
}

func TestParseMatchersUnsupportedType(t *testing.T) {
	t.Parallel()

	h := createDummy(t) // default: only Exact + URITemplate

	query := url.Values{"matchFooBar": {"pattern"}}
	_, err := h.parseMatchers(query, false)
	assert.ErrorIs(t, err, ErrUnsupportedMatcherType)
}

func TestParseMatchersNotImplementedWhenTypeDisabled(t *testing.T) {
	t.Parallel()

	// Explicitly register only Exact — URLPattern is deliberately absent to
	// verify the unsupported-type branch; the implicit defaults are bypassed
	// by passing at least one WithMatcherType.
	h := createDummy(t, WithMatcherType("Exact", ExactMatcher))

	query := url.Values{"matchURLPattern": {"https://example.com/:id"}}
	_, err := h.parseMatchers(query, false)
	assert.ErrorIs(t, err, ErrUnsupportedMatcherType)
}

func TestParseMatchersURLPatternRejectsRelative(t *testing.T) {
	t.Parallel()

	h := createDummy(t, withAllMatcherTypes()...)

	query := url.Values{"matchURLPattern": {"/books/:id"}}
	_, err := h.parseMatchers(query, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "URLPattern")
	assert.Contains(t, err.Error(), "relative URL")
}

func TestParseMatchersMissing(t *testing.T) {
	t.Parallel()

	h := createDummy(t)

	query := url.Values{"other": {"value"}}
	_, err := h.parseMatchers(query, false)
	assert.Error(t, err)
}

func TestParseMatchersMixed(t *testing.T) {
	t.Parallel()

	h := createDummy(t, withAllMatcherTypes()...)

	// Mix of different matcher types
	query := url.Values{
		"match":           {"exact-topic"},
		"matchURLPattern": {"https://example.com/:id"},
		"matchRegexp":     {"https://example\\.com/books/[0-9]+"},
		"authorization":   {"some-jwt"}, // Non-match param should be ignored
		"lastEventID":     {"some-id"},  // Non-match param should be ignored
	}

	matchers, err := h.parseMatchers(query, false)
	require.NoError(t, err)

	assert.Len(t, matchers, 3)
}

func TestParseMatchersNonMatchParamsIgnored(t *testing.T) {
	t.Parallel()

	h := createDummy(t, withAllMatcherTypes()...)

	query := url.Values{
		"match":         {"foo"},
		"authorization": {"jwt"},
		"lastEventID":   {"id"},
	}

	matchers, err := h.parseMatchers(query, false)
	require.NoError(t, err)

	assert.Len(t, matchers, 1)
}
