package hub

import (
	"strings"
	"sync"

	"github.com/yosida95/uritemplate"
)

type selector struct {
	sync.RWMutex
	// counter stores the number of subsribers currently using this topic
	counter uint32
	// the uritemplate.Template instance, of nil if it's a raw string
	template   *uritemplate.Template
	matchCache map[string]bool
}

// topicSelectorStore caches uritemplate.Template to improve memory and CPU usage.
type topicSelectorStore struct {
	sync.RWMutex
	m map[string]*selector
}

func newTopicSelectorStore() *topicSelectorStore {
	return &topicSelectorStore{m: make(map[string]*selector)}
}

func (tss *topicSelectorStore) match(topic, topicSelector string, addToCache bool) bool {
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

	match = templateStore.template != nil && templateStore.template.Match(topic) != nil
	templateStore.Lock()
	templateStore.matchCache[topic] = match
	templateStore.Unlock()

	return match
}

// getTemplateStore retrieves or creates the uritemplate.Template associated with this topic, or nil if it's not a template.
func (tss *topicSelectorStore) getTemplateStore(topicSelector string, addToCache bool) *selector {
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
	if strings.Contains(topicSelector, "{") { // If it's definitely not an URI template, skip to save some resources
		s.template, _ = uritemplate.New(topicSelector) // Returns nil in case of error, will be considered as a raw string
	}

	if addToCache {
		tss.m[topicSelector] = s
	}

	return s
}

// Remove unused uritemplate.Template instances from memory.
func (tss *topicSelectorStore) cleanup(topics []string) {
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
