package mercure

import (
	"testing"
	"time"

	"github.com/dgraph-io/ristretto"
	"github.com/stretchr/testify/assert"
)

func TestMatchRistretto(t *testing.T) {
	cache, _ := ristretto.NewCache(&ristretto.Config{
		NumCounters: TopicSelectorStoreRistrettoDefaultCacheNumCounters,
		MaxCost:     TopicSelectorStoreRistrettoCacheMaxCost,
		BufferItems: 64,
	})
	tss := &TopicSelectorStore{cache, false}

	assert.True(t, tss.match("https://example.com/foo/bar", "https://example.com/{foo}/bar"))

	// wait for value to pass through ristretto's buffers
	cache.Wait()

	_, found := tss.cache.Get("t_https://example.com/{foo}/bar")
	assert.True(t, found)

	_, found = tss.cache.Get("m_https://example.com/{foo}/bar_https://example.com/foo/bar")
	assert.True(t, found)

	assert.True(t, tss.match("https://example.com/foo/bar", "https://example.com/{foo}/bar"))
	assert.False(t, tss.match("https://example.com/foo/bar/baz", "https://example.com/{foo}/bar"))

	// wait for value to pass through ristretto's buffers, see https://discuss.dgraph.io/t/there-should-be-a-test-only-blocking-mode/8424
	time.Sleep(10 * time.Millisecond)

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
