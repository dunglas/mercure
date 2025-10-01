package mercure

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchCache(t *testing.T) {
	t.Parallel()

	tss, err := NewTopicSelectorStoreCache(DefaultTopicSelectorStoreCacheMaxEntriesPerShard, DefaultTopicSelectorStoreCacheMaxEntriesPerShard)
	require.NoError(t, err)

	assert.False(t, tss.match("foo", "bar"))

	assert.True(t, tss.match("https://example.com/foo/bar", "https://example.com/{foo}/bar"))

	_, found := tss.cache.Get("t_https://example.com/{foo}/bar")
	assert.True(t, found)

	_, found = tss.cache.Get("m_https://example.com/{foo}/bar_https://example.com/foo/bar")
	assert.True(t, found)

	assert.True(t, tss.match("https://example.com/foo/bar", "https://example.com/{foo}/bar"))
	assert.False(t, tss.match("https://example.com/foo/bar/baz", "https://example.com/{foo}/bar"))

	_, found = tss.cache.Get("t_https://example.com/{foo}/bar")
	assert.True(t, found)

	_, found = tss.cache.Get("m_https://example.com/{foo}/bar_https://example.com/foo/bar")
	assert.True(t, found)

	assert.True(t, tss.match("https://example.com/kevin/dunglas", "https://example.com/{fistname}/{lastname}"))
	assert.True(t, tss.match("https://example.com/foo/bar", "*"))
	assert.True(t, tss.match("https://example.com/foo/bar", "https://example.com/foo/bar"))
	assert.True(t, tss.match("foo", "foo"))
	assert.False(t, tss.match("foo", "bar"))
}
