package mercure

import (
	"regexp"
	"strings"

	uritemplate "github.com/yosida95/uritemplate/v3"
)

type TopicSelectorStoreCache interface {
	Get(key interface{}) (interface{}, bool)
	Set(key interface{}, value interface{}, n int64) bool
}

// TopicSelectorStore caches compiled templates to improve memory and CPU usage.
type TopicSelectorStore struct {
	cache      TopicSelectorStoreCache
	skipSelect bool
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
