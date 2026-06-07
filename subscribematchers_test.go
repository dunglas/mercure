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

	query := url.Values{"topic": {"foo", "bar"}}
	matchers, err := h.parseMatchers(query, false)
	require.NoError(t, err)

	assert.Len(t, matchers, 2)
	assert.Equal(t, MatcherTypeExact, matchers[0].Type)
	assert.Equal(t, "foo", matchers[0].Pattern)
	assert.Equal(t, MatcherTypeExact, matchers[1].Type)
	assert.Equal(t, "bar", matchers[1].Pattern)
}

func TestParseMatchersURLPattern(t *testing.T) {
	t.Parallel()

	h := createDummy(t)

	query := url.Values{"topicURLPattern": {"https://example.com/:id"}}
	matchers, err := h.parseMatchers(query, false)
	require.NoError(t, err)

	require.Len(t, matchers, 1)
	assert.Equal(t, MatcherTypeURLPattern, matchers[0].Type)
}

// TestParseMatchersCaseSensitive verifies the spec rule: topic matcher query
// parameter names are case-sensitive, and a request using any other matcher
// parameter name must be rejected.
func TestParseMatchersCaseSensitive(t *testing.T) {
	t.Parallel()

	h := createDummy(t)

	for _, name := range []string{"TOPIC", "Topic", "topicUrlPattern", "TOPICURLPATTERN", "topicRegexp"} {
		_, err := h.parseMatchers(url.Values{name: {"foo"}}, false)
		assert.ErrorIs(t, err, errUnknownMatcherParam, name)
	}
}

func TestParseMatchersInvalidURLPattern(t *testing.T) {
	t.Parallel()

	h := createDummy(t)

	_, err := h.parseMatchers(url.Values{"topicURLPattern": {"{unclosed"}}, false)
	require.Error(t, err)
}

func TestParseMatchersInvalidValue(t *testing.T) {
	t.Parallel()

	h := createDummy(t)

	for _, v := range []string{"foo\x00bar", "foo\nbar", "\x7f", "\xff"} {
		_, err := h.parseMatchers(url.Values{"topic": {v}}, false)
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

	_, err := h.parseMatchers(url.Values{"topic": tooMany}, false)
	require.ErrorIs(t, err, errTooManyMatchers)

	_, err = h.parseMatchers(url.Values{"topic": {strings.Repeat("a", maxPatternLength+1)}}, false)
	require.ErrorIs(t, err, errPatternTooLong)
}

// TestParseMatchersURLPatternRelativeAccepted exercises the spec rule that
// allows relative URL patterns (anchored at the hub URL). This shape is also
// used by the hub's own subscription-events stream.
func TestParseMatchersURLPatternRelativeAccepted(t *testing.T) {
	t.Parallel()

	h := createDummy(t)

	query := url.Values{"topicURLPattern": {"/.well-known/mercure/subscriptions/:matchType/:match/:subscriber"}}
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
	assert.ErrorIs(t, err, errMissingTopic)
}

func TestParseMatchersNonMatcherParamsIgnored(t *testing.T) {
	t.Parallel()

	h := createDummy(t)

	query := url.Values{
		"topic":         {"foo"},
		"authorization": {"jwt"},
		"lastEventID":   {"id"},
	}

	matchers, err := h.parseMatchers(query, false)
	require.NoError(t, err)

	assert.Len(t, matchers, 1)
}
