package mercure

import (
	"errors"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"regexp/syntax"
	"slices"
	"strings"

	urlpattern "github.com/dunglas/go-urlpattern"
	"github.com/maypok86/otter/v2"
)

// DefaultTopicMatcherStoreCacheSize bounds the (matcher_type, pattern, topics)
// -> bool match cache and each compiled-pattern cache. At ~100 B/entry,
// 100_000 keeps each cache under ~10 MB.
// Raise it via the `topic_matcher_cache <N>` Caddyfile directive for hubs
// handling a much larger topic / matcher universe.
const DefaultTopicMatcherStoreCacheSize = 100_000

// Weighting by retained bytes prevents large keys from exhausting the cache budget.
const avgMatchCacheEntrySize = 100

func matchCacheEntryWeight(k matchCacheKey, _ bool) uint32 {
	const fixedOverhead = 64 // Estimated cache bookkeeping per entry.

	weight := fixedOverhead + len(k.Base) + len(k.Type) + len(k.Pattern) + len(k.Topics)

	return uint32(min(uint64(weight), math.MaxUint32))
}

// Heap upper bounds measured with runtime.MemStats; (a{1000}) compiles to 1000 copies, so length alone is unsafe.
const (
	urlPatternOverhead   = 8 << 10 // One compiled regexp per URL component.
	urlPatternByteWeight = 256
	regexpInstWeight     = 64
	regexpRuneWeight     = 16
)

const maxURLPatternWeight = 2 << 20

var errURLPatternTooComplex = errors.New("pattern too complex")

// compiled carries its weight so it is estimated once, before deciding whether to cache.
type compiled[T any] struct {
	value  T
	weight uint32
}

func compiledWeight[T any](_ string, c compiled[T]) uint32 {
	return c.weight
}

func regexpWeight(re *syntax.Regexp) uint64 {
	insts, runes := regexpSize(re)

	return insts*regexpInstWeight + runes*regexpRuneWeight
}

// One-pass regexps copy rune tables for repeated instructions.
func regexpSize(re *syntax.Regexp) (insts, runes uint64) {
	runes = uint64(len(re.Rune))

	var sub uint64

	for _, s := range re.Sub {
		i, r := regexpSize(s)
		sub += i
		runes += r
	}

	switch {
	case re.Op == syntax.OpLiteral:
		insts = uint64(len(re.Rune))
	case re.Op == syntax.OpRepeat && re.Max >= 0:
		insts = uint64(re.Max) * (sub + 1)
		runes *= uint64(re.Max)
	case re.Op == syntax.OpRepeat && re.Min > 0:
		insts = 1 + uint64(re.Min)*sub
		runes *= uint64(re.Min)
	default:
		insts = 2 + sub + uint64(len(re.Sub))
	}

	return max(1, insts), runes
}

// urlPatternWeight reparses regexp groups because urlpattern does not expose its compiled regexps.
func urlPatternWeight(pattern string) uint64 {
	weight := urlPatternOverhead + uint64(len(pattern))*urlPatternByteWeight

	for _, group := range urlPatternRegexpGroups(pattern) {
		re, err := syntax.Parse(group, syntax.Perl)
		if err != nil {
			return math.MaxUint32
		}

		// A repeated group appears twice in the generated regexp.
		weight += 2 * regexpWeight(re)
	}

	return weight
}

// urlPatternRegexpGroups delimits regexp groups as the URL Pattern tokenizer does.
func urlPatternRegexpGroups(pattern string) []string {
	var groups []string

	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '\\':
			i++
		case '(':
			start, depth := i+1, 1

		group:
			for i = start; i < len(pattern); i++ {
				switch pattern[i] {
				case '\\':
					i++
				case '(':
					depth++
				case ')':
					if depth--; depth == 0 {
						break group
					}
				}
			}

			groups = append(groups, pattern[start:min(i, len(pattern))])
		}
	}

	return groups
}

// topicsKeySeparator joins the topics of an update into a single cache-key
// field. It is a NUL byte, which the publish and subscribe handlers reject in
// topics before they can reach the cache; no escaping is required here.
const topicsKeySeparator = "\x00"

// urlPatternFallbackBase is the base URL applied when no public URL is
// configured. ".invalid" is reserved by RFC 6761 §6.4, so it cannot collide
// with a real absolute pattern. Its path is the hub URL, as the protocol
// requires of the base, so a path-relative reference resolves here as it does
// against a configured resource identifier and in the reserved-namespace guard,
// which shares this base (see reservedtopic.go).
// Relative ↔ relative matching is identity-preserving against any consistent
// base, so subscription events (which use relative topics) match correctly even
// without configuration. Cross-mode matching (a relative pattern against an
// absolute topic on the hub URL or vice versa) requires a real base URL —
// configure a resource identifier ending in "/.well-known/mercure"
// (WithResourceIdentifier / `resource_identifier`), which then doubles as the
// base.
const urlPatternFallbackBase = "http://mercure.invalid" + defaultHubURL

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
	matchCache          *otter.Cache[matchCacheKey, bool]
	templateCache       *otter.Cache[string, compiled[*regexp.Regexp]]
	urlPatterns         *otter.Cache[string, compiled[*urlpattern.URLPattern]]
	compiledCacheWeight uint64

	baseURL string
}

// NewTopicMatcherStore creates a TopicMatcherStore.
// If cacheSize > 0, match results, compiled templates and compiled URL
// patterns are cached; otherwise nothing is memoised.
func NewTopicMatcherStore(cacheSize int) (*TopicMatcherStore, error) {
	if cacheSize <= 0 {
		return &TopicMatcherStore{}, nil
	}

	weight := uint64(cacheSize) * avgMatchCacheEntrySize

	matchCache, err := otter.New(&otter.Options[matchCacheKey, bool]{
		MaximumWeight: weight,
		Weigher:       matchCacheEntryWeight,
	})
	if err != nil {
		return nil, err //nolint:wrapcheck
	}

	templateCache, err := otter.New(&otter.Options[string, compiled[*regexp.Regexp]]{
		MaximumWeight: weight,
		Weigher:       compiledWeight[*regexp.Regexp],
	})
	if err != nil {
		return nil, err //nolint:wrapcheck
	}

	urlPatterns, err := otter.New(&otter.Options[string, compiled[*urlpattern.URLPattern]]{
		MaximumWeight: weight,
		Weigher:       compiledWeight[*urlpattern.URLPattern],
	})
	if err != nil {
		return nil, err //nolint:wrapcheck
	}

	return &TopicMatcherStore{
		matchCache:          matchCache,
		templateCache:       templateCache,
		urlPatterns:         urlPatterns,
		compiledCacheWeight: weight,
	}, nil
}

// cacheCompiled skips values heavier than the whole budget, which would evict everything else.
func cacheCompiled[T any](c *otter.Cache[string, compiled[T]], budget uint64, key string, value T, weight uint64) {
	if weight > budget || weight > math.MaxUint32 {
		return
	}

	c.Set(key, compiled[T]{value: value, weight: uint32(weight)})
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
	case MatcherTypeExact:
		return nil
	case deprecatedMatcherTypeName:
		return tms.validateDeprecated(m.Pattern)
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
			return cached.value, nil
		}
	}

	// Checked before compiling: the allocation itself is the amplification.
	weight := uint64(len(base)) + urlPatternWeight(pattern)
	if weight > maxURLPatternWeight {
		return nil, fmt.Errorf("invalid URL pattern: %w", errURLPatternTooComplex)
	}

	// A nil Options keeps ignoreCase disabled, as mandated by the protocol.
	p, err := urlpattern.New(pattern, base, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid URL pattern: %w", err)
	}

	if tms.urlPatterns != nil {
		cacheCompiled(tms.urlPatterns, tms.compiledCacheWeight, key, p, weight)
	}

	return p, nil
}
