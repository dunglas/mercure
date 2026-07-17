package mercure

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"

	urlpattern "github.com/dunglas/go-urlpattern"
	"github.com/maypok86/otter/v2"
)

// DefaultTopicMatcherStoreCacheSize bounds the (matcher_type, pattern, topics)
// -> bool match cache. At ~100 B/entry, 100_000 keeps the cache under ~10 MB.
// Raise it via the `topic_matcher_cache <N>` Caddyfile directive for hubs
// handling a much larger topic / matcher universe.
const DefaultTopicMatcherStoreCacheSize = 100_000

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
// allocating. Base scopes the entry to the base URL patterns were resolved
// against, so a store shared across hubs with different base URLs never serves
// a result computed under the wrong base.
type matchCacheKey struct {
	Base    string
	Type    MatcherType
	Pattern string
	Topics  string
}

// TopicMatcherStore caches match results and compiled patterns. The match
// cache is a single unsharded otter instance; otter v2 is designed for high
// concurrency.
type TopicMatcherStore struct {
	matchCache    *otter.Cache[matchCacheKey, bool]
	templateCache *otter.Cache[string, *regexp.Regexp]
	urlPatterns   *otter.Cache[string, *urlpattern.URLPattern]

	baseURL string
}

// NewTopicMatcherStore creates a TopicMatcherStore.
// If cacheSize > 0, match results, compiled templates and compiled URL
// patterns are cached; otherwise nothing is memoised.
func NewTopicMatcherStore(cacheSize int) (*TopicMatcherStore, error) {
	if cacheSize <= 0 {
		return &TopicMatcherStore{}, nil
	}

	matchCache, err := otter.New(&otter.Options[matchCacheKey, bool]{
		MaximumSize: cacheSize,
	})
	if err != nil {
		return nil, err //nolint:wrapcheck
	}

	// Compiled templates and URL patterns are fewer but larger than match
	// results. Size them at a fraction of the match cache, with a floor of 1:
	// otter treats MaximumSize == 0 as unbounded, which would let an attacker
	// stream distinct patterns until OOM.
	auxSize := max(cacheSize/10, 1)

	templateCache, err := otter.New(&otter.Options[string, *regexp.Regexp]{
		MaximumSize: auxSize,
	})
	if err != nil {
		return nil, err //nolint:wrapcheck
	}

	urlPatterns, err := otter.New(&otter.Options[string, *urlpattern.URLPattern]{
		MaximumSize: auxSize,
	})
	if err != nil {
		return nil, err //nolint:wrapcheck
	}

	return &TopicMatcherStore{matchCache: matchCache, templateCache: templateCache, urlPatterns: urlPatterns}, nil
}

// ErrConflictingBaseURL is returned by setBaseURL (via NewHub) when a store
// already configured with one base URL is reused by a hub with a different
// public URL. The base URL is immutable configuration; sharing a store across
// hubs that disagree on it would silently corrupt relative-pattern matching, so
// it is rejected at construction instead.
var ErrConflictingBaseURL = errors.New("topic matcher store already configured with a different base URL")

// ErrInvalidBaseURL is returned by setBaseURL (via NewHub) when the configured
// public URL is not an absolute URL. Relative URL patterns and topics are
// resolved against it, so an invalid value would make every relative-pattern
// match fail at request time with an opaque error; it is rejected at
// construction instead.
var ErrInvalidBaseURL = errors.New("base URL must be an absolute URL")

// setBaseURL sets the base URL used to resolve relative URL patterns and
// topics, per the protocol's "the hub MUST use the hub's URL as the base URL"
// rule. Must be called before the hub starts serving requests: compiled
// patterns embed the base. Setting the same value again, or an empty value, is
// a no-op; changing an already-set base URL is rejected.
func (tms *TopicMatcherStore) setBaseURL(baseURL string) error {
	if baseURL == "" || baseURL == tms.baseURL {
		return nil
	}

	if tms.baseURL != "" {
		return fmt.Errorf("%w: %q vs %q", ErrConflictingBaseURL, tms.baseURL, baseURL)
	}

	if u, err := url.Parse(baseURL); err != nil || !u.IsAbs() || u.Host == "" {
		return fmt.Errorf("%w: %q", ErrInvalidBaseURL, baseURL)
	}

	tms.baseURL = baseURL

	return nil
}

// base returns the configured base URL, falling back to a synthetic origin —
// only relative ↔ relative and absolute ↔ absolute matches work in that case.
func (tms *TopicMatcherStore) base() string {
	if tms.baseURL == "" {
		return urlPatternFallbackBase
	}

	return tms.baseURL
}

// validatePattern compiles the pattern up front so invalid patterns surface
// as a 400 / 401 instead of silently matching nothing.
func (tms *TopicMatcherStore) validatePattern(m TopicMatcher) error {
	switch m.Type {
	case MatcherTypeExact, deprecatedMatcherTypeName:
		// Any string is a valid exact pattern; v8 selectors that are not
		// valid URI Templates fall back to exact comparison.
		return nil
	case MatcherTypeURLPattern:
		_, err := tms.getOrCompileURLPattern(m.Pattern)

		return err
	default:
		return ErrUnsupportedMatcherType
	}
}

// matches dispatches matching per matcher type, caching results of
// non-trivial matchers per (type, pattern, topic-set).
func (tms *TopicMatcherStore) matches(topics []string, m TopicMatcher) bool {
	// "*" is the reserved wildcard: it matches every topic regardless of
	// matcher type, so a topic literally equal to "*" is not addressable.
	if m.Pattern == "*" {
		return true
	}

	switch m.Type {
	case MatcherTypeExact:
		// Exact matching is so fast it doesn't need caching.
		return slices.Contains(topics, m.Pattern)
	case MatcherTypeURLPattern:
		return tms.cachedMatch(topics, m, tms.matchURLPattern)
	case deprecatedMatcherTypeName:
		return tms.matchDeprecated(topics, m)
	default:
		return false
	}
}

// cachedMatch runs fn through the match cache.
func (tms *TopicMatcherStore) cachedMatch(topics []string, m TopicMatcher, fn func([]string, string) bool) bool {
	if tms.matchCache == nil {
		return fn(topics, m.Pattern)
	}

	k := matchCacheKey{Base: tms.base(), Type: m.Type, Pattern: m.Pattern, Topics: strings.Join(topics, topicsKeySeparator)}
	if v, ok := tms.matchCache.GetIfPresent(k); ok {
		return v
	}

	r := fn(topics, m.Pattern)
	tms.matchCache.Set(k, r)

	return r
}

func (tms *TopicMatcherStore) matchURLPattern(topics []string, pattern string) bool {
	p, err := tms.getOrCompileURLPattern(pattern)
	if err != nil {
		return false
	}

	base := tms.base()

	return slices.ContainsFunc(topics, func(t string) bool { return p.Test(t, base) })
}

func (tms *TopicMatcherStore) getOrCompileURLPattern(pattern string) (*urlpattern.URLPattern, error) {
	base := tms.base()
	// Compiled patterns embed the base URL, so the cache key must include it:
	// a store shared across hubs with different base URLs would otherwise reuse
	// a pattern compiled against the wrong base. The base is a URL and cannot
	// contain NUL, so it is an unambiguous key prefix.
	key := base + topicsKeySeparator + pattern

	if tms.urlPatterns != nil {
		if cached, ok := tms.urlPatterns.GetIfPresent(key); ok {
			return cached, nil
		}
	}

	// A nil Options keeps ignoreCase disabled, as mandated by the protocol.
	p, err := urlpattern.New(pattern, base, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid URL pattern: %w", err)
	}

	if tms.urlPatterns != nil {
		tms.urlPatterns.Set(key, p)
	}

	return p, nil
}
