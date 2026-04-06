package mercure

import (
	"slices"
	"strings"

	"github.com/maypok86/otter/v2"
)

// DefaultTopicSelectorStoreCacheSize bounds the (matcher_type, pattern, topics)
// -> bool match cache. At ~100 B/entry, 100_000 keeps the cache under ~10 MB.
// Raise it via the `topic_selector_cache <N>` Caddyfile directive for hubs
// handling a much larger topic / selector universe.
const DefaultTopicSelectorStoreCacheSize = 100_000

// topicsKeySeparator joins the topics of an update into a single cache-key
// field. It is a NUL byte, which can never appear in an IRI nor in a topic
// identifier defined by the Mercure protocol, so no escaping is required.
const topicsKeySeparator = "\x00"

// matchCacheKey is the comparable struct used as the match-cache key. The
// Topics field holds the update's topics joined with a NUL byte; for the
// common single-topic case, strings.Join returns the single element without
// allocating.
type matchCacheKey struct {
	Type    string
	Pattern string
	Topics  string
}

// registeredMatcher bundles a resolved matcher with the canonical name it was
// registered under, so subscription events can emit the type in the same case
// the operator used (e.g. `URLPattern`, not `urlpattern`).
type registeredMatcher struct {
	canonicalName string
	matcher       Matcher
}

// TopicSelectorStore caches match results and holds the registry of available
// matcher types. The cache is a single unsharded otter instance; otter v2 is
// designed for high concurrency.
type TopicSelectorStore struct {
	matchCache *otter.Cache[matchCacheKey, bool]
	matchers   map[string]registeredMatcher // lowercase name → canonical name + implementation
}

// NewTopicSelectorStore creates a TopicSelectorStore. If cacheSize > 0, match
// results are cached. Compiled patterns (regexps, URL patterns, CEL programs,
// URI templates) are always memoised inside each matcher implementation.
func NewTopicSelectorStore(cacheSize int) (*TopicSelectorStore, error) {
	if cacheSize <= 0 {
		return &TopicSelectorStore{}, nil
	}

	matchCache, err := otter.New[matchCacheKey, bool](&otter.Options[matchCacheKey, bool]{
		MaximumSize: cacheSize,
	})
	if err != nil {
		return nil, err //nolint:wrapcheck
	}

	return &TopicSelectorStore{matchCache: matchCache}, nil
}

// RegisterMatcherType registers a matcher type by name. Lookups are
// case-insensitive; the casing provided here is the canonical form used in
// serialized subscription events and subscription API payloads.
//
// RegisterMatcherType must be called during hub setup, before the hub starts
// serving requests. The matcher registry is not protected by a mutex for
// performance; concurrent registration and lookup would race.
func (tss *TopicSelectorStore) RegisterMatcherType(name string, mt Matcher) {
	if tss.matchers == nil {
		tss.matchers = make(map[string]registeredMatcher)
	}

	tss.matchers[strings.ToLower(name)] = registeredMatcher{canonicalName: name, matcher: mt}
}

// ResolveMatcherType looks up a matcher type by name (case-insensitive).
func (tss *TopicSelectorStore) ResolveMatcherType(name string) (Matcher, bool) { //nolint:ireturn
	if tss.matchers == nil {
		return nil, false
	}

	rm, ok := tss.matchers[strings.ToLower(name)]

	return rm.matcher, ok
}

// newTopicMatcher creates a topicMatcher with the matcher implementation resolved.
// The resulting Type field carries the canonical casing the matcher was
// registered under (not the caller's casing), so wire-format representations
// stay consistent across requests.
func (tss *TopicSelectorStore) newTopicMatcher(typeName, pattern string) (topicMatcher, error) {
	if tss.matchers == nil {
		return topicMatcher{}, ErrUnsupportedMatcherType
	}

	rm, ok := tss.matchers[strings.ToLower(typeName)]
	if !ok {
		return topicMatcher{}, ErrUnsupportedMatcherType
	}

	return topicMatcher{
		Type:    rm.canonicalName,
		Pattern: pattern,
		matcher: rm.matcher,
	}, nil
}

// matchMatcher dispatches matching to the resolved matcher implementation,
// caching the result per (type, pattern, topic-set).
func (tss *TopicSelectorStore) matchMatcher(topics []string, m topicMatcher) bool {
	// Wildcard always matches.
	if m.Pattern == "*" {
		return true
	}

	// Defensive: unresolved matchers never match. Normal flow (newTopicMatcher
	// / resolveMatcherClaims) guarantees matcher is non-nil, but callers that
	// pass hand-built topicMatcher values may reach this point without one.
	if m.matcher == nil {
		return false
	}

	// Exact matching is so fast it doesn't need caching.
	if _, ok := m.matcher.(exactMatcherType); ok {
		return slices.Contains(topics, m.Pattern)
	}

	var k matchCacheKey

	if tss.matchCache != nil {
		k = matchCacheKey{Type: m.Type, Pattern: m.Pattern, Topics: strings.Join(topics, topicsKeySeparator)}
		if v, ok := tss.matchCache.GetIfPresent(k); ok {
			return v
		}
	}

	r := m.matcher.Match(topics, m.Pattern)

	if tss.matchCache != nil {
		tss.matchCache.Set(k, r)
	}

	return r
}
