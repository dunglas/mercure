package mercure

import (
	"hash/fnv"

	lru "github.com/hashicorp/golang-lru"
)

// Gather stats to find the best default values.
const (
	DefaultTopicSelectorStoreLRUMaxEntriesPerShard = int64(1e4)
	DefaultTopicSelectorStoreLRUShardCount         = int64(256) // 2.5 million entries.
)

// NewTopicSelectorStoreLRU creates a TopicSelectorStore with an LRU cache.
func NewTopicSelectorStoreLRU(maxEntriesPerShard, shardCount int64) (*TopicSelectorStore, error) {
	if maxEntriesPerShard == 0 {
		return &TopicSelectorStore{}, nil
	}
	if shardCount == 0 {
		shardCount = DefaultTopicSelectorStoreLRUShardCount
	}

	lruMap := make(shardedLRUCache, shardCount)
	for i := 0; i < int(shardCount); i++ {
		lruMap[i], _ = lru.New(int(maxEntriesPerShard))
	}

	return &TopicSelectorStore{cache: &lruMap, skipSelect: true}, nil
}

type shardedLRUCache map[int]*lru.Cache

func (c *shardedLRUCache) Get(k interface{}) (interface{}, bool) {
	return c.getShard(k).Get(k)
}

func (c *shardedLRUCache) Set(k interface{}, v interface{}, _ int64) bool {
	c.getShard(k).Add(k, v)

	return true
}

func (c *shardedLRUCache) getShard(k interface{}) *lru.Cache {
	h := fnv.New32a()
	h.Write([]byte(k.(string)))

	return (*c)[int(h.Sum32())%len(*c)]
}
