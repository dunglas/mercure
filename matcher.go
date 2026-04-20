package mercure

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// ErrUnsupportedMatcherType is returned when a matcher type is not registered.
// The HTTP handler maps this to a 501 "Not Implemented" status code.
var ErrUnsupportedMatcherType = errors.New("unsupported topic matcher type")

// ErrStringClaimRequiresCompat is returned when a JWT mercure.publish /
// mercure.subscribe claim uses the legacy string form in modern mode. The
// v9+ protocol requires the object form {match, matchType, payload}; the
// string form is accepted only under WithProtocolVersionCompatibility.
// The HTTP handler maps this to a 401 "Unauthorized" status code.
var ErrStringClaimRequiresCompat = errors.New("string-form matcher claims require backward-compatibility mode")

// Matcher defines how a topic pattern is matched.
// Implement this interface to add custom matcher types.
//
// Match reports whether the given topics match the pattern. The full topic
// list is passed because some matchers (notably CEL) have aggregate semantics
// that cannot be expressed one topic at a time. Per-topic matchers should
// iterate the slice and return true on the first hit. Matchers that benefit
// from pattern compilation (regexp, URL patterns, …) should cache compiled
// patterns internally; the TopicSelectorStore caches Match results keyed
// by (type, pattern, topic-set).
type Matcher interface {
	Match(topics []string, pattern string) bool
}

// topicMatcher pairs a resolved matcher implementation with a pattern string.
type topicMatcher struct {
	Type    string  // Matcher type name ("Exact", "URLPattern", etc.)
	Pattern string  // The pattern to match against
	matcher Matcher // Resolved implementation, set at parse time
}

// matcherClaim represents a single entry in the mercure.publish or mercure.subscribe JWT claim.
// It supports both the legacy string format and the new object format.
type matcherClaim struct {
	topicMatcher

	Payload any // Per-subscription payload, nil if not set
}

// MarshalJSON serializes a matcherClaim back to JSON.
// String claims (legacy or simple patterns) are serialized as plain strings.
// Object claims are serialized as {"match": ..., "matchType": ..., "payload": ...}.
func (mc *matcherClaim) MarshalJSON() ([]byte, error) {
	// Legacy string claims and unresolved claims → plain string
	if mc.Type == "" || mc.Type == legacyMatcherTypeName {
		b, err := json.Marshal(mc.Pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal matcher claim pattern: %w", err)
		}

		return b, nil
	}

	obj := struct {
		Match     string `json:"match"`
		MatchType string `json:"matchType,omitempty"`
		Payload   any    `json:"payload,omitempty"`
	}{
		Match:     mc.Pattern,
		MatchType: mc.Type,
		Payload:   mc.Payload,
	}

	b, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal matcher claim object: %w", err)
	}

	return b, nil
}

// UnmarshalJSON handles both string and object formats in JWT claims.
// String: treated according to protocol version (Exact for v9+, legacy for v8-).
// Object: {"match": "pattern", "matchType": "Exact", "payload": {...}}.
func (mc *matcherClaim) UnmarshalJSON(data []byte) error {
	// Try string first (most common for backward compat)
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		mc.Pattern = s
		// Type and matcher are resolved later based on protocol version
		// Empty Type signals "unresolved string claim"
		mc.Type = ""

		return nil
	}

	// Try object format
	var obj struct {
		Match     string `json:"match"`
		MatchType string `json:"matchType"`
		Payload   any    `json:"payload"`
	}

	if err := json.Unmarshal(data, &obj); err != nil {
		return err //nolint:wrapcheck
	}

	mc.Pattern = obj.Match
	mc.Payload = obj.Payload

	if obj.MatchType == "" {
		mc.Type = exactMatcherTypeName
	} else {
		mc.Type = strings.ToLower(obj.MatchType)
	}

	return nil
}

// writeMatcherClaimError writes the appropriate HTTP status for an error
// returned by resolveMatcherClaims: 501 for unknown matcher types, 401 for
// everything else (malformed string claim, compat violation, …).
func writeMatcherClaimError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrUnsupportedMatcherType) {
		http.Error(w, http.StatusText(http.StatusNotImplemented), http.StatusNotImplemented)

		return
	}

	http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
}

// resolveMatcherClaims resolves the matcher implementation for each claim entry.
//
// String-form entries (Type == "") are only permitted under legacy mode,
// where they map to the v8 "exact OR URI Template" rule. In modern mode
// the v9+ protocol requires the object form, so string entries are
// rejected with ErrStringClaimRequiresCompat — silently reinterpreting
// them as Exact would change the meaning of tokens minted for v8.
// Object-form entries look their type up in the registry.
//
// The function is idempotent: claims whose matcher is already resolved are
// skipped, so callers may run it repeatedly without re-validating.
func resolveMatcherClaims(tss *TopicSelectorStore, claims []matcherClaim, legacy bool) error {
	for i := range claims {
		if claims[i].matcher != nil {
			continue // already resolved
		}

		if claims[i].Type == "" {
			if !legacy {
				return ErrStringClaimRequiresCompat
			}

			claims[i].Type = legacyMatcherTypeName
			claims[i].matcher = legacyMatcher

			continue
		}

		if claims[i].Type == legacyMatcherTypeName {
			// Already typed as legacy (e.g. by stringsToLegacyMatchers); plug
			// the matcher in without a registry lookup since "_legacy" is
			// intentionally not publicly registered.
			claims[i].matcher = legacyMatcher

			continue
		}

		if tss.matchers == nil {
			return ErrUnsupportedMatcherType
		}

		rm, ok := tss.matchers[strings.ToLower(claims[i].Type)]
		if !ok {
			return ErrUnsupportedMatcherType
		}

		// Canonicalise the claim's type so it matches the casing the hub
		// uses everywhere else (subscription events, subscription API, …).
		claims[i].Type = rm.canonicalName
		claims[i].matcher = rm.matcher
	}

	return nil
}

const (
	// exactMatcherTypeName is the name of the built-in exact matcher type.
	exactMatcherTypeName = "exact"

	// legacyMatcherTypeName is used internally for backward-compatible string claims.
	legacyMatcherTypeName = "_legacy"
)

// matcherClaimsToMatchers extracts the topicMatchers from a slice of matcherClaims.
func matcherClaimsToMatchers(claims []matcherClaim) []topicMatcher {
	if claims == nil {
		return nil
	}

	matchers := make([]topicMatcher, len(claims))
	for i, c := range claims {
		matchers[i] = c.topicMatcher
	}

	return matchers
}

// stringsToLegacyMatchers wraps a slice of v8-style string topic selectors
// into legacyMatcher-backed topicMatchers. It is the single shared builder
// used by the legacy `topic` query parameter and the tests that exercise
// v8 behaviour.
func stringsToLegacyMatchers(patterns []string) []topicMatcher {
	if patterns == nil {
		return nil
	}

	out := make([]topicMatcher, len(patterns))
	for i, p := range patterns {
		out[i] = topicMatcher{Type: legacyMatcherTypeName, Pattern: p, matcher: legacyMatcher}
	}

	return out
}

// stringsToLegacyClaims wraps a slice of v8-style string topic selectors into
// matcherClaim entries ready to be stored in JWT mercure.{publish,subscribe}
// claims. Shares the underlying conversion with stringsToLegacyMatchers.
func stringsToLegacyClaims(patterns []string) []matcherClaim {
	matchers := stringsToLegacyMatchers(patterns)

	claims := make([]matcherClaim, len(matchers))
	for i, m := range matchers {
		claims[i] = matcherClaim{topicMatcher: m}
	}

	return claims
}
