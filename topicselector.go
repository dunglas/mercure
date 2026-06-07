package mercure

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"

	urlpattern "github.com/dunglas/go-urlpattern"
	"github.com/maypok86/otter/v2"
)

// DefaultTopicSelectorStoreCacheSize bounds the (matcher_type, pattern, topics)
// -> bool match cache. At ~100 B/entry, 100_000 keeps the cache under ~10 MB.
// Raise it via the `topic_selector_cache <N>` Caddyfile directive for hubs
// handling a much larger topic / matcher universe.
const DefaultTopicSelectorStoreCacheSize = 100_000

// topicsKeySeparator joins the topics of an update into a single cache-key
// field. It is a NUL byte, which the publish and subscribe handlers reject in
// topics before they can reach the cache; no escaping is required here.
const topicsKeySeparator = "\x00"

// urlPatternFallbackBase is the base URL applied when no public URL is
// configured. ".invalid" is reserved by RFC 6761 §6.4, so it cannot collide
// with a real absolute pattern. Relative ↔ relative matching is
// identity-preserving against any consistent base, so subscription events
// (which use relative topics) match correctly even without configuration.
// Cross-mode matching (a relative pattern against an absolute topic on the
// hub URL or vice versa) requires the real public URL — set it with
// WithPublicURL (Go) or `public_url` (Caddyfile).
const urlPatternFallbackBase = "http://mercure.invalid"

// matchCacheKey is the comparable struct used as the match-cache key. The
// Topics field holds the update's topics joined with a NUL byte; for the
// common single-topic case, strings.Join returns the single element without
// allocating.
type matchCacheKey struct {
	Type    MatcherType
	Pattern string
	Topics  string
}

// TopicSelectorStore caches match results and compiled patterns. The match
// cache is a single unsharded otter instance; otter v2 is designed for high
// concurrency.
type TopicSelectorStore struct {
	matchCache    *otter.Cache[matchCacheKey, bool]
	templateCache *otter.Cache[string, *regexp.Regexp]

	baseURL     string
	urlPatterns sync.Map // pattern string → *urlpattern.URLPattern
}

// NewTopicSelectorStore creates a TopicSelectorStore.
// If cacheSize > 0, match results are cached. Compiled URL patterns are
// always memoised.
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

	templateCache, err := otter.New[string, *regexp.Regexp](&otter.Options[string, *regexp.Regexp]{
		MaximumSize: cacheSize / 10, // Templates are fewer but larger
	})
	if err != nil {
		return nil, err //nolint:wrapcheck
	}

	return &TopicSelectorStore{matchCache: matchCache, templateCache: templateCache}, nil
}

// setBaseURL sets the base URL used to resolve relative URL patterns and
// topics, per the protocol's "the hub MUST use the hub's URL as the base URL"
// rule. Must be called before the hub starts serving requests: compiled
// patterns embed the base.
func (tss *TopicSelectorStore) setBaseURL(baseURL string) {
	tss.baseURL = baseURL
}

// base returns the configured base URL, falling back to a synthetic origin —
// only relative ↔ relative and absolute ↔ absolute matches work in that case.
func (tss *TopicSelectorStore) base() string {
	if tss.baseURL == "" {
		return urlPatternFallbackBase
	}

	return tss.baseURL
}

// validatePattern compiles the pattern up front so invalid patterns surface
// as a 400 / 401 instead of silently matching nothing.
func (tss *TopicSelectorStore) validatePattern(m topicMatcher) error {
	switch m.Type {
	case MatcherTypeExact, deprecatedMatcherTypeName:
		// Any string is a valid exact pattern; v8 selectors that are not
		// valid URI Templates fall back to exact comparison.
		return nil
	case MatcherTypeURLPattern:
		_, err := tss.getOrCompileURLPattern(m.Pattern)

		return err
	default:
		return ErrUnsupportedMatcherType
	}
}

// matchMatcher dispatches matching per matcher type, caching results of
// non-trivial matchers per (type, pattern, topic-set).
func (tss *TopicSelectorStore) matchMatcher(topics []string, m topicMatcher) bool {
	// Wildcard always matches.
	if m.Pattern == "*" {
		return true
	}

	switch m.Type {
	case MatcherTypeExact:
		// Exact matching is so fast it doesn't need caching.
		return slices.Contains(topics, m.Pattern)
	case MatcherTypeURLPattern:
		return tss.cachedMatch(topics, m, tss.matchURLPattern)
	case deprecatedMatcherTypeName:
		return tss.matchDeprecatedMatcher(topics, m)
	default:
		return false
	}
}

// cachedMatch runs fn through the match cache.
func (tss *TopicSelectorStore) cachedMatch(topics []string, m topicMatcher, fn func([]string, string) bool) bool {
	if tss.matchCache == nil {
		return fn(topics, m.Pattern)
	}

	k := matchCacheKey{Type: m.Type, Pattern: m.Pattern, Topics: strings.Join(topics, topicsKeySeparator)}
	if v, ok := tss.matchCache.GetIfPresent(k); ok {
		return v
	}

	r := fn(topics, m.Pattern)
	tss.matchCache.Set(k, r)

	return r
}

func (tss *TopicSelectorStore) matchURLPattern(topics []string, pattern string) bool {
	p, err := tss.getOrCompileURLPattern(pattern)
	if err != nil {
		return false
	}

	base := tss.base()

	return slices.ContainsFunc(topics, func(t string) bool { return p.Test(t, base) })
}

func (tss *TopicSelectorStore) getOrCompileURLPattern(pattern string) (*urlpattern.URLPattern, error) {
	if cached, ok := tss.urlPatterns.Load(pattern); ok {
		return cached.(*urlpattern.URLPattern), nil
	}

	// A nil Options keeps ignoreCase disabled, as mandated by the protocol.
	p, err := urlpattern.New(pattern, tss.base(), nil)
	if err != nil {
		return nil, fmt.Errorf("invalid URL pattern: %w", err)
	}

	actual, _ := tss.urlPatterns.LoadOrStore(pattern, p)

	return actual.(*urlpattern.URLPattern), nil
}
