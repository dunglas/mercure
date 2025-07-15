package mercure

import (
	"hash/fnv"

	lru "github.com/hashicorp/golang-lru"
)

// Gather stats to find the best default values.
const (
	DefaultTopicSelectorStoreLRUMaxEntriesPerShard = int(1e4)
	DefaultTopicSelectorStoreLRUShardCount         = int(256) // 2.5 million entries.
)

// NewTopicSelectorStoreLRU creates a TopicSelectorStore with an LRU cache.
func NewTopicSelectorStoreLRU(maxEntriesPerShard, shardCount int) (*TopicSelectorStore, error) {
	if maxEntriesPerShard == 0 {
		return &TopicSelectorStore{}, nil
	}

	if shardCount == 0 {
		shardCount = DefaultTopicSelectorStoreLRUShardCount
	}

	lruMap := make(shardedLRUCache, shardCount)
	for i := 0; i < shardCount; i++ {
		lruMap[i], _ = lru.New(maxEntriesPerShard)
	}

	return &TopicSelectorStore{cache: &lruMap, skipSelect: true}, nil
}

type shardedLRUCache map[int]*lru.Cache

func (c *shardedLRUCache) Get(k string) (interface{}, bool) {
	return c.getShard(k).Get(k)
}

func (c *shardedLRUCache) Set(k string, v interface{}, _ int64) bool {
	c.getShard(k).Add(k, v)

	return true
}

func (c *shardedLRUCache) getShard(k interface{}) *lru.Cache {
	h := fnv.New32a()
	_, _ = h.Write([]byte(k.(string)))

	return (*c)[int(h.Sum32())%len(*c)]
}
