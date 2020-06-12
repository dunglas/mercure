package hub

import (
	"regexp"
	"strings"
	"sync"

	"github.com/yosida95/uritemplate"
)

type selector struct {
	sync.RWMutex
	// counter stores the number of subsribers currently using this topic
	counter uint32
	// the regexp.Regexp instance, of nil if it's a raw string
	regexp     *regexp.Regexp
	matchCache map[string]bool
}

// topicSelectorStore caches compiled templates to improve memory and CPU usage.
type TopicSelectorStore struct {
	sync.RWMutex
	m map[string]*selector
}

// NewTopicSelectorStore creates a new topic selector store.
func NewTopicSelectorStore() *TopicSelectorStore {
	return &TopicSelectorStore{m: make(map[string]*selector)}
}

func (tss *TopicSelectorStore) match(topic, topicSelector string, addToCache bool) bool {
	// Always do an exact matching comparison first
	// Also check if the topic selector is the reserved keyword *
	if topicSelector == "*" || topic == topicSelector {
		return true
	}

	templateStore := tss.getTemplateStore(topicSelector, addToCache)
	templateStore.RLock()
	match, ok := templateStore.matchCache[topic]
	templateStore.RUnlock()
	if ok {
		return match
	}

	// Use template.Regexp() instead of template.Match() for performance
	// See https://github.com/yosida95/uritemplate/pull/7
	match = templateStore.regexp != nil && templateStore.regexp.MatchString(topic)
	templateStore.Lock()
	templateStore.matchCache[topic] = match
	templateStore.Unlock()

	return match
}

// getTemplateStore retrieves or creates the compiled template associated with this topic, or nil if it's not a template.
func (tss *TopicSelectorStore) getTemplateStore(topicSelector string, addToCache bool) *selector {
	if addToCache {
		tss.Lock()
		defer tss.Unlock()
	} else {
		tss.RLock()
	}

	s, ok := tss.m[topicSelector]
	if !addToCache {
		tss.RUnlock()
	}
	if ok {
		if addToCache {
			s.counter++
		}

		return s
	}

	s = &selector{matchCache: make(map[string]bool)}
	// If it's definitely not an URI template, skip to save some resources
	if strings.Contains(topicSelector, "{") {
		// If an error occurs, it's a raw string
		if tpl, err := uritemplate.New(topicSelector); err == nil {
			s.regexp = tpl.Regexp()
		}
	}

	if addToCache {
		tss.m[topicSelector] = s
	}

	return s
}

// cleanup removes unused compiled templates from memory.
func (tss *TopicSelectorStore) cleanup(topics []string) {
	tss.Lock()
	defer tss.Unlock()
	for _, topic := range topics {
		if tc, ok := tss.m[topic]; ok {
			if tc.counter == 0 {
				delete(tss.m, topic)
				continue
			}

			tc.counter--
		}
	}
}
