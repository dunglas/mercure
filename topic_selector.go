package mercure

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/dgraph-io/ristretto"
	uritemplate "github.com/yosida95/uritemplate/v3"
)

// TopicSelectorStore caches compiled templates to improve memory and CPU usage.
type TopicSelectorStore struct {
	cache *ristretto.Cache
}

// NewTopicSelectorStore creates a TopicSelectorStore instance.
// See https://github.com/dgraph-io/ristretto, defaults to 6e7 counters and 100MB of max cost, set values to -1 to disable.
func NewTopicSelectorStore(cacheNumCounters, cacheMaxCost int64) (*TopicSelectorStore, error) {
	if cacheNumCounters == -1 {
		return &TopicSelectorStore{}, nil
	}

	if cacheNumCounters == 0 {
		cacheNumCounters = 6e7 // gather stats to find the best default values
	}
	if cacheMaxCost == 0 {
		cacheMaxCost = 1e8
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

	r := tss.getRegexp(topicSelector)
	if r == nil {
		return false
	}

	var k string
	if tss.cache != nil {
		k = "m_" + topicSelector + "_" + topic
		value, found := tss.cache.Get(k)
		if found {
			return value.(bool)
		}
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
