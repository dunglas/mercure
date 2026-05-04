package mercure

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegexpMatcher(t *testing.T) {
	t.Parallel()

	m := regexpMatcherType{}

	// Simple patterns
	assert.True(t, m.Match([]string{"foo"}, "foo"))
	assert.True(t, m.Match([]string{"foo"}, ".*"))
	assert.True(t, m.Match([]string{"foo"}, "f.o"))
	assert.True(t, m.Match([]string{"foobar"}, "foo.*"))

	// Character classes
	assert.True(t, m.Match([]string{"foo123"}, "[a-z]+[0-9]+"))
	assert.False(t, m.Match([]string{"FOO"}, "[a-z]+"))

	// Alternation
	assert.True(t, m.Match([]string{"foo"}, "foo|bar"))
	assert.True(t, m.Match([]string{"bar"}, "foo|bar"))
	assert.False(t, m.Match([]string{"baz"}, "foo|bar"))

	// URL-like patterns
	assert.True(t, m.Match([]string{"https://example.com/books/123"}, "https://example\\.com/books/[0-9]+"))
	assert.False(t, m.Match([]string{"https://example.com/books/abc"}, "https://example\\.com/books/[0-9]+"))

	// I-Regexp is anchored (full match, not partial)
	assert.False(t, m.Match([]string{"foobar"}, "foo"))
	assert.False(t, m.Match([]string{"barfoo"}, "foo"))

	// Wildcard-like
	assert.True(t, m.Match([]string{"anything"}, ".*"))
	assert.True(t, m.Match([]string{""}, ".*"))

	// No match
	assert.False(t, m.Match([]string{"foo"}, "bar"))

	// Multiple topics — at least one matches
	assert.True(t, m.Match([]string{"baz", "foo"}, "foo"))
	assert.False(t, m.Match([]string{"baz", "qux"}, "foo"))
}

func TestValidateIRegexp(t *testing.T) {
	t.Parallel()

	// Valid patterns
	assert.NoError(t, validateIRegexp(".*"))
	assert.NoError(t, validateIRegexp("foo"))
	assert.NoError(t, validateIRegexp("[a-z]+"))
	assert.NoError(t, validateIRegexp("foo|bar"))
	assert.NoError(t, validateIRegexp("https://example\\.com/.*"))
	assert.NoError(t, validateIRegexp("\\d+"))
	// Escaped anchors are literals.
	assert.NoError(t, validateIRegexp("\\^foo\\$"))
	// `^` inside a character class is the negation, not an anchor.
	assert.NoError(t, validateIRegexp("[^abc]+"))
	// Literal `$` is allowed inside a character class.
	assert.NoError(t, validateIRegexp("[$]"))

	// Invalid: anchors at the boundaries
	require.Error(t, validateIRegexp("^foo"))
	require.Error(t, validateIRegexp("foo$"))
	// Invalid: anchors mid-pattern (Go's regex would silently honour these
	// and the user's intent — a literal '$' or '^' — would be lost).
	require.Error(t, validateIRegexp("foo$bar"))
	require.Error(t, validateIRegexp("(foo|^bar)"))

	// Invalid: backreferences
	assert.Error(t, validateIRegexp("(foo)\\1"))
}

func TestRegexpMatcherInvalidPattern(t *testing.T) {
	t.Parallel()

	m := regexpMatcherType{}

	// Invalid regexp syntax
	assert.False(t, m.Match([]string{"foo"}, "[invalid"))

	// Invalid I-Regexp (with anchors)
	assert.False(t, m.Match([]string{"foo"}, "^foo$"))
}
