package mercure

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCELMatcherMatch(t *testing.T) {
	t.Parallel()

	m := newCELMatcherType(nil)

	// Simple topic check using topics.exists()
	assert.True(t, m.Match(
		[]string{"https://example.com/foo"},
		`topics.exists(t, t == "https://example.com/foo")`,
	))

	// No match
	assert.False(t, m.Match(
		[]string{"https://example.com/bar"},
		`topics.exists(t, t == "https://example.com/foo")`,
	))

	// Multiple topics — at least one matches
	assert.True(t, m.Match(
		[]string{"https://example.com/bar", "https://example.com/foo"},
		`topics.exists(t, t == "https://example.com/foo")`,
	))

	// Check topic starts with prefix
	assert.True(t, m.Match(
		[]string{"https://example.com/books/123"},
		`topics.exists(t, t.startsWith("https://example.com/books/"))`,
	))
	assert.False(t, m.Match(
		[]string{"https://example.com/users/123"},
		`topics.exists(t, t.startsWith("https://example.com/books/"))`,
	))

	// Check topics size
	assert.True(t, m.Match(
		[]string{"a", "b", "c"},
		`topics.size() > 2`,
	))
	assert.False(t, m.Match(
		[]string{"a"},
		`topics.size() > 2`,
	))

	// Always true
	assert.True(t, m.Match([]string{"anything"}, "true"))

	// Always false
	assert.False(t, m.Match([]string{"anything"}, "false"))
}

func TestCELMatcherInvalidExpression(t *testing.T) {
	t.Parallel()

	m := newCELMatcherType(nil)

	// Syntax error
	assert.False(t, m.Match([]string{"foo"}, "invalid!!!"))

	// Compile error: wrong return type
	_, err := m.compile(`"not a bool"`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must return bool")

	// Compile error: undefined variable
	_, err = m.compile("undefined_var")
	assert.Error(t, err)
}

// TestCELMatcherCostLimit protects the hub from DoS via expensive CEL expressions
// submitted by untrusted clients. Match() must return false (eval aborted) instead
// of burning CPU on a pathological expression.
func TestCELMatcherCostLimit(t *testing.T) {
	t.Parallel()

	m := newCELMatcherType(nil)

	// Build a long topic list so nested comprehensions blow past the cost limit.
	topics := make([]string, 500)
	for i := range topics {
		topics[i] = "t"
	}

	// O(n^3) over 500 topics = 125M — far beyond celEvaluationCostLimit.
	assert.False(t, m.Match(topics, `topics.all(a, topics.all(b, topics.all(c, a == b && b == c)))`))
}

// TestCELMatcherSeesAggregateTopics exercises the CEL-specific behaviour that
// relies on the full topic list being passed through (topics.size(), etc.).
func TestCELMatcherSeesAggregateTopics(t *testing.T) {
	t.Parallel()

	m := newCELMatcherType(nil)

	assert.True(t, m.Match([]string{"a", "b", "c"}, `topics.size() == 3`))
	assert.False(t, m.Match([]string{"a"}, `topics.size() == 3`))
}
