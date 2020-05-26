package hub

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatch(t *testing.T) {
	tss := newTopicSelectorStore()

	assert.True(t, tss.match("https://example.com/foo/bar", "https://example.com/{foo}/bar", false))
	assert.Empty(t, tss.m)
	assert.True(t, tss.match("https://example.com/foo/bar", "https://example.com/{foo}/bar", true))
	assert.False(t, tss.match("https://example.com/foo/bar/baz", "https://example.com/{foo}/bar", true))
	assert.NotNil(t, tss.m["https://example.com/{foo}/bar"].regexp)
	assert.True(t, tss.m["https://example.com/{foo}/bar"].matchCache["https://example.com/foo/bar"])
	assert.False(t, tss.m["https://example.com/{foo}/bar"].matchCache["https://example.com/foo/bar/baz"])
	assert.Equal(t, tss.m["https://example.com/{foo}/bar"].counter, uint32(1))

	assert.True(t, tss.match("https://example.com/kevin/dunglas", "https://example.com/{fistname}/{lastname}", true))
	assert.True(t, tss.match("https://example.com/foo/bar", "*", true))
	assert.True(t, tss.match("https://example.com/foo/bar", "https://example.com/foo/bar", true))
	assert.True(t, tss.match("foo", "foo", true))
	assert.False(t, tss.match("foo", "bar", true))

	tss.cleanup([]string{"https://example.com/{foo}/bar", "https://example.com/{fistname}/{lastname}", "bar"})
	assert.Len(t, tss.m, 1)

	tss.cleanup([]string{"https://example.com/{foo}/bar", "https://example.com/{fistname}/{lastname}"})
	assert.Empty(t, tss.m)
}
