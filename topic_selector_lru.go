package mercure

import (
	"hash/fnv"

	"github.com/hashicorp/golang-lru"
)

// Gather stats to find the best default values.
const (
	DefaultTopicSelectorStoreLruMaxEntriesPerShard = int64(1e4)
	DefaultTopicSelectorStoreLruShardCount         = int64(256) // 2.5 million entries.
)

// NewTopicSelectorStoreLru creates a TopicSelectorStore with an lru cache
func NewTopicSelectorStoreLru(maxEntriesPerShard, shardCount int64) (*TopicSelectorStore, error) {
	if maxEntriesPerShard == 0 {
		return &TopicSelectorStore{}, nil
	}
	if shardCount == 0 {
		shardCount = DefaultTopicSelectorStoreLruShardCount
	}
	lruMap := make(shardedLruCache, shardCount)
	for i := 0; i < int(shardCount); i++ {
		lruMap[i], _ = lru.New(int(maxEntriesPerShard))
	}

	return &TopicSelectorStore{cache: &lruMap, skipSelect: true}, nil
}

type shardedLruCache map[int]*lru.Cache

func (c *shardedLruCache) Get(k interface{}) (interface{}, bool) {
	return c.getShard(k).Get(k)
}

func (c *shardedLruCache) Set(k interface{}, v interface{}, _ int64) bool {
	c.getShard(k).Add(k, v)
	return true
}

func (c *shardedLruCache) getShard(k interface{}) *lru.Cache {
	h := fnv.New32a()
	h.Write([]byte(k.(string)))
	return (*c)[int(h.Sum32())%len(*c)]
}
