package mercure

import (
	"fmt"

	"github.com/dgraph-io/ristretto"
)

// Gather stats to find the best default values.
const (
	TopicSelectorStoreRistrettoDefaultCacheNumCounters = int64(6e7)
	TopicSelectorStoreRistrettoCacheMaxCost            = int64(1e8) // 100 MB
)

// NewTopicSelectorStoreRistretto creates a TopicSelectorStore instance with a ristretto cache.
// See https://github.com/dgraph-io/ristretto, set values to 0 to disable.
//
// Deprecated: use NewTopicSelectorStoreLRU instead.
func NewTopicSelectorStoreRistretto(cacheNumCounters, cacheMaxCost int64) (*TopicSelectorStore, error) {
	if cacheNumCounters == 0 {
		return &TopicSelectorStore{}, nil
	}

	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: cacheNumCounters,
		MaxCost:     cacheMaxCost,
		BufferItems: 64,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to create cache: %w", err)
	}

	return &TopicSelectorStore{cache: cache}, nil
}
