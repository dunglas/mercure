package mercure

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/dgraph-io/ristretto"
	uritemplate "github.com/yosida95/uritemplate/v3"
)

// Gather stats to find the best default values.
const (
	TopicSelectorStoreDefaultCacheNumCounters = int64(6e7)
	TopicSelectorStoreCacheMaxCost            = int64(1e8) // 100 MB
)

type TopicSelectorStoreCache interface {
	Get(interface{}) (interface{}, bool)
	Set(interface{}, interface{}, int64) bool
}

// TopicSelectorStore caches compiled templates to improve memory and CPU usage.
type TopicSelectorStore struct {
	cache      TopicSelectorStoreCache
	skipSelect bool
}

// NewTopicSelectorStore creates a TopicSelectorStore instance with a ristretto cache.
// See https://github.com/dgraph-io/ristretto, set values to 0 to disable.
func NewTopicSelectorStore(cacheNumCounters, cacheMaxCost int64) (*TopicSelectorStore, error) {
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

func (tss *TopicSelectorStore) match(topic, topicSelector string) bool {
	// Always do an exact matching comparison first
	// Also check if the topic selector is the reserved keyword *
	if topicSelector == "*" || topic == topicSelector {
		return true
	}

	var k string
	if tss.cache != nil {
		k = "m_" + topicSelector + "_" + topic
		value, found := tss.cache.Get(k)
		if found {
			return value.(bool)
		}
	}

	r := tss.getRegexp(topicSelector)
	if r == nil {
		return false
	}

	// Use template.Regexp() instead of template.Match() for performance
	// See https://github.com/yosida95/uritemplate/pull/7
	match := r.MatchString(topic)
	if tss.cache != nil {
		tss.cache.Set(k, match, 4)
	}

	return match
}

// getRegexp retrieves regexp for this template selector.
func (tss *TopicSelectorStore) getRegexp(topicSelector string) *regexp.Regexp {
	// If it's definitely not an URI template, skip to save some resources
	if !strings.Contains(topicSelector, "{") {
		return nil
	}

	var k string
	if tss.cache != nil {
		k = "t_" + topicSelector
		value, found := tss.cache.Get(k)
		if found {
			return value.(*regexp.Regexp)
		}
	}

	// If an error occurs, it's a raw string
	if tpl, err := uritemplate.New(topicSelector); err == nil {
		r := tpl.Regexp()
		if tss.cache != nil {
			tss.cache.Set(k, r, 19)
		}

		return r
	}

	return nil
}
