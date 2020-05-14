package hub

import (
	"strings"
	"sync"

	"github.com/yosida95/uritemplate"
)

type selector struct {
	sync.Mutex
	// counter stores the number of subsribers currently using this topic
	counter uint32
	// the uritemplate.Template instance, of nil if it's a raw string
	template   *uritemplate.Template
	matchCache map[string]bool
}

// topicSelectorStore caches uritemplate.Template to improve memory and CPU usage.
type topicSelectorStore struct {
	sync.Mutex
	m map[string]*selector
}

func newTopicSelectorStore() *topicSelectorStore {
	return &topicSelectorStore{m: make(map[string]*selector)}
}

func (tss *topicSelectorStore) match(topic, topicSelector string, addToCache bool) bool {
	// Always do an exact matching comparision first
	// Also check if the topic selector is the reserved keyword *
	if topicSelector == "*" || topic == topicSelector {
		return true
	}

	templateStore := tss.getTemplateStore(topicSelector, addToCache)
	templateStore.Lock()
	defer templateStore.Unlock()
	if match, ok := templateStore.matchCache[topic]; ok {
		return match
	}

	match := templateStore.template != nil && templateStore.template.Match(topic) != nil
	templateStore.matchCache[topic] = match

	return match
}

// getTemplateStore retrieves or creates the uritemplate.Template associated with this topic, or nil if it's not a template.
func (tss *topicSelectorStore) getTemplateStore(topicSelector string, addToCache bool) (s *selector) {
	tss.Lock()
	defer tss.Unlock()
	if store, ok := tss.m[topicSelector]; ok {
		if addToCache {
			store.counter++
		}

		return store
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
			if tc.counter <= 0 {
				delete(tss.m, topic)
			} else {
				tc.counter--
			}
		}
	}
}
