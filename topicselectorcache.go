package mercure

import (
	"hash/fnv"

	"github.com/maypok86/otter/v2"
)

// Let's say that a topic selector is 100 bytes on average, a cache with
// 10,000 entries per shard and 256 shards will use about 256 * 10,000 * 100 = 256MB of RAM.
// TODO: gather stats to find the best default values.
const (
	DefaultTopicSelectorStoreCacheMaxEntriesPerShard = int(10_000)
	DefaultTopicSelectorStoreCacheShardCount         = int(256)
)

// NewTopicSelectorStoreCache creates a TopicSelectorStore with a cache.
func NewTopicSelectorStoreCache(maxEntriesPerShard, shardCount int) (*TopicSelectorStore, error) {
	if maxEntriesPerShard == 0 {
		return &TopicSelectorStore{}, nil
	}

	if shardCount == 0 {
		shardCount = DefaultTopicSelectorStoreCacheShardCount
	}

	cacheMap := make(shardedCache, shardCount)
	for i := 0; i < shardCount; i++ {
		cacheMap[i] = otter.Must(&otter.Options[string, any]{MaximumSize: maxEntriesPerShard})
	}

	return &TopicSelectorStore{cache: &cacheMap, skipSelect: true}, nil
}

type shardedCache map[int]*otter.Cache[string, any]

func (c *shardedCache) Get(k string) (any, bool) {
	return c.getShard(k).GetIfPresent(k)
}

func (c *shardedCache) Set(k string, v any, _ int64) bool {
	c.getShard(k).Set(k, v)

	return true
}

func (c *shardedCache) getShard(k string) *otter.Cache[string, any] {
	h := fnv.New32a()
	_, _ = h.Write([]byte(k))

	return (*c)[int(h.Sum32())%len(*c)]
}
