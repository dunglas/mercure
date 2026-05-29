package mercure

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestURITemplateMatcher(t *testing.T) {
	t.Parallel()

	m := uriTemplateMatcherType{}

	// Single variable
	assert.True(t, m.Match([]string{"https://example.com/foo/bar"}, "https://example.com/{foo}/bar"))
	assert.False(t, m.Match([]string{"https://example.com/foo/bar/baz"}, "https://example.com/{foo}/bar"))

	// Multiple variables
	assert.True(t, m.Match([]string{"https://example.com/kevin/dunglas"}, "https://example.com/{firstname}/{lastname}"))

	// Query parameter variable
	assert.True(t, m.Match([]string{"https://example.com/users/foo/?topic=bar"}, "https://example.com/users/foo/{?topic}"))

	// Exact match (not a template)
	assert.True(t, m.Match([]string{"https://example.com/foo/bar"}, "https://example.com/foo/bar"))

	// No match
	assert.False(t, m.Match([]string{"https://example.com/baz"}, "https://example.com/foo"))

	// Invalid template
	assert.False(t, m.Match([]string{"foo"}, "invalid{"))

	// Multiple topics — at least one matches
	assert.True(t, m.Match([]string{"https://example.com/baz", "https://example.com/foo/bar"}, "https://example.com/{foo}/bar"))
}
