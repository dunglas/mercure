package mercure

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMatchersExact(t *testing.T) {
	t.Parallel()

	h := createDummy(t)

	// Bare "match" defaults to the Exact matcher type.
	query := url.Values{"match": {"foo", "bar"}}
	matchers, err := h.parseMatchers(query, false)
	require.NoError(t, err)

	assert.Len(t, matchers, 2)
	assert.Equal(t, MatcherTypeExact, matchers[0].Type)
	assert.Equal(t, "foo", matchers[0].Pattern)
	assert.Equal(t, MatcherTypeExact, matchers[1].Type)
	assert.Equal(t, "bar", matchers[1].Pattern)
}

func TestParseMatchersExactExplicit(t *testing.T) {
	t.Parallel()

	h := createDummy(t)

	// "matchExact" is the explicit spelling of the default Exact type.
	query := url.Values{"matchExact": {"foo"}}
	matchers, err := h.parseMatchers(query, false)
	require.NoError(t, err)

	require.Len(t, matchers, 1)
	assert.Equal(t, MatcherTypeExact, matchers[0].Type)
	assert.Equal(t, "foo", matchers[0].Pattern)
}

func TestParseMatchersURLPattern(t *testing.T) {
	t.Parallel()

	h := createDummy(t)

	query := url.Values{"matchURLPattern": {"https://example.com/:id"}}
	matchers, err := h.parseMatchers(query, false)
	require.NoError(t, err)

	require.Len(t, matchers, 1)
	assert.Equal(t, MatcherTypeURLPattern, matchers[0].Type)
}

// TestParseMatchersCaseSensitive verifies the spec rule: topic matcher query
// parameter names are case-sensitive, and a request using any other parameter
// name in the reserved "match" namespace (an unknown matcher type or a case
// typo of a known name) must be rejected.
func TestParseMatchersCaseSensitive(t *testing.T) {
	t.Parallel()

	h := createDummy(t)

	for _, name := range []string{"MATCH", "Match", "matchurlpattern", "MATCHURLPATTERN", "matchExactly", "matchRegexp"} {
		_, err := h.parseMatchers(url.Values{name: {"foo"}}, false)
		assert.ErrorIs(t, err, errUnknownMatcherParam, name)
	}
}

// TestParseMatchersTopicRejectedInModernMode verifies the deprecated v8 "topic"
// query parameter is rejected outside compatibility mode, with a migration
// hint pointing at "match".
func TestParseMatchersTopicRejectedInModernMode(t *testing.T) {
	t.Parallel()

	h := createDummy(t)

	_, err := h.parseMatchers(url.Values{"topic": {"foo"}}, false)
	assert.ErrorIs(t, err, errUnknownMatcherParam)
}

func TestParseMatchersInvalidURLPattern(t *testing.T) {
	t.Parallel()

	h := createDummy(t)

	_, err := h.parseMatchers(url.Values{"matchURLPattern": {"{unclosed"}}, false)
	require.Error(t, err)
}

func TestParseMatchersInvalidValue(t *testing.T) {
	t.Parallel()

	h := createDummy(t)

	// Covers C0 (NUL, LF), DEL (U+007F), a C1 control (U+0085, valid UTF-8 but
	// a control character) and invalid UTF-8 (\xff).
	for _, v := range []string{"foo\x00bar", "foo\nbar", "\x7f", "\u0085", "\xff"} {
		_, err := h.parseMatchers(url.Values{"match": {v}}, false)
		assert.ErrorIs(t, err, errInvalidMatcherValue)
	}
}

func TestParseMatchersLimits(t *testing.T) {
	t.Parallel()

	h := createDummy(t)

	tooMany := make([]string, maxMatcherCount+1)
	for i := range tooMany {
		tooMany[i] = "foo"
	}

	_, err := h.parseMatchers(url.Values{"match": tooMany}, false)
	require.ErrorIs(t, err, errTooManyMatchers)

	_, err = h.parseMatchers(url.Values{"match": {strings.Repeat("a", maxPatternLength+1)}}, false)
	require.ErrorIs(t, err, errPatternTooLong)
}

// TestParseMatchersURLPatternRelativeAccepted exercises the spec rule that
// allows relative URL patterns (anchored at the hub URL). This shape is also
// used by the hub's own subscription-events stream.
func TestParseMatchersURLPatternRelativeAccepted(t *testing.T) {
	t.Parallel()

	h := createDummy(t)

	query := url.Values{"matchURLPattern": {"/.well-known/mercure/subscriptions/:matchType/:match/:subscriber"}}
	matchers, err := h.parseMatchers(query, false)
	require.NoError(t, err)
	require.Len(t, matchers, 1)
	assert.Equal(t, MatcherTypeURLPattern, matchers[0].Type)
}

func TestParseMatchersMissing(t *testing.T) {
	t.Parallel()

	h := createDummy(t)

	query := url.Values{"other": {"value"}}
	_, err := h.parseMatchers(query, false)
	assert.ErrorIs(t, err, errMissingMatcher)
}

func TestParseMatchersNonMatcherParamsIgnored(t *testing.T) {
	t.Parallel()

	h := createDummy(t)

	query := url.Values{
		"match":         {"foo"},
		"authorization": {"jwt"},
		"lastEventID":   {"id"},
	}

	matchers, err := h.parseMatchers(query, false)
	require.NoError(t, err)

	assert.Len(t, matchers, 1)
}
