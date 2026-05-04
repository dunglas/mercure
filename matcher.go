package mercure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
)

// ErrUnsupportedMatcherType is returned when a matcher type is not registered.
// The HTTP handler maps this to a 501 "Not Implemented" status code.
var ErrUnsupportedMatcherType = errors.New("unsupported topic matcher type")

// errStringClaimRequiresCompat is returned when a JWT mercure.publish /
// mercure.subscribe claim uses the deprecated string form in modern mode. The
// v9+ protocol requires the object form; the string form is accepted only
// under WithProtocolVersionCompatibility. Mapped to 401 on the wire.
var errStringClaimRequiresCompat = errors.New("string-form matcher claims require backward-compatibility mode")

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

// PatternValidator is an optional interface that Matcher implementations can
// implement to validate a pattern up front. When implemented, the hub calls
// Validate at subscription parse time (and during JWT claim resolution) so
// invalid patterns surface as a 400 / 401 instead of silently matching nothing.
type PatternValidator interface {
	Validate(pattern string) error
}

// validatePattern runs Validate on the matcher if it implements
// PatternValidator. Matchers that don't implement the interface accept any
// pattern at parse time and report mismatches by returning false from Match.
func validatePattern(m Matcher, pattern string) error {
	v, ok := m.(PatternValidator)
	if !ok {
		return nil
	}

	return v.Validate(pattern)
}

// topicMatcher pairs a resolved matcher implementation with a pattern string.
type topicMatcher struct {
	Type    string  // Matcher type name ("Exact", "URLPattern", etc.)
	Pattern string  // The pattern to match against
	matcher Matcher // Resolved implementation, set at parse time
}

// matcherClaim represents a single entry in the mercure.publish or
// mercure.subscribe JWT claim. It supports both the deprecated string format and
// the new object format.
type matcherClaim struct {
	topicMatcher

	Payload any // Per-subscription payload, nil if not set
}

// MarshalJSON serialises a claim back to the wire format: a plain string for
// deprecated/unresolved entries, an object otherwise. Used when a hub signs
// its own JWTs in tests.
func (mc *matcherClaim) MarshalJSON() ([]byte, error) {
	if mc.Type == "" || mc.Type == deprecatedMatcherTypeName {
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
	}{mc.Pattern, mc.Type, mc.Payload}

	b, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal matcher claim object: %w", err)
	}

	return b, nil
}

// UnmarshalJSON handles both string and object formats in JWT claims.
// String: treated according to protocol version (Exact for v9+, deprecated for v8-).
// Object: {"match": "pattern", "matchType": "Exact", "payload": {...}}.
//
// Always resets every field of the receiver before populating it, so
// reusing a matcherClaim across decode calls does not leak the previous
// Type/Payload/matcher (which would, in particular, make
// resolveMatcherClaims skip resolution entirely on a stale matcher).
func (mc *matcherClaim) UnmarshalJSON(data []byte) error {
	*mc = matcherClaim{}

	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		// Empty Type signals "unresolved string claim"; resolveMatcherClaims
		// decides what it means based on the protocol version.
		mc.Pattern = s

		return nil
	}

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

// writeMatcherClaimError translates a resolveMatcherClaims error into an
// HTTP response: 501 for unknown matcher types, 401 for everything else
// (string claim in modern mode, malformed claim, …). It also logs the
// cause at info level so operators upgrading from v8 see a hint without
// having to enable debug logging.
func writeMatcherClaimError(ctx context.Context, logger *slog.Logger, w http.ResponseWriter, err error) {
	if errors.Is(err, ErrUnsupportedMatcherType) {
		http.Error(w, http.StatusText(http.StatusNotImplemented), http.StatusNotImplemented)
	} else {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
	}

	if logger == nil || !logger.Enabled(ctx, slog.LevelInfo) {
		return
	}

	switch {
	case errors.Is(err, errStringClaimRequiresCompat):
		logger.LogAttrs(ctx, slog.LevelInfo,
			`JWT contains v8 bare-string topic claims. Re-mint tokens with the {"match": "...", "matchType": "..."} object form, or run the hub with WithProtocolVersionCompatibility(8) and the deprecated_topic build tag to keep accepting them.`,
			slog.Any("error", err))
	case errors.Is(err, ErrUnsupportedMatcherType):
		logger.LogAttrs(ctx, slog.LevelInfo,
			"JWT references a matcher type that is not registered on this hub. Register it with WithMatcherType (Go) or matcher_types (Caddyfile).",
			slog.Any("error", err))
	default:
		logger.LogAttrs(ctx, slog.LevelInfo,
			"Failed to resolve JWT topic matcher claims",
			slog.Any("error", err))
	}
}

// resolveMatcherClaims resolves the matcher implementation for each claim.
//
// String-form entries (Type == "") are only permitted under deprecated mode,
// where they map to the v8 "exact OR URI Template" rule. In modern mode the
// v9+ protocol requires the object form; silently reinterpreting bare
// strings as Exact would change the meaning of tokens minted for v8.
//
// The function is idempotent: already-resolved claims are skipped, so
// callers may run it repeatedly without re-validating.
//
// JWT claims are untrusted input, so the same maxMatcherCount and
// maxPatternLength caps the query parser enforces also apply here —
// otherwise a token can drive resolution into multi-megabyte
// allocations or pathological matcher compilations.
func resolveMatcherClaims(tss *TopicSelectorStore, claims []matcherClaim, deprecated bool) error {
	if len(claims) > maxMatcherCount {
		return errTooManyMatchers
	}

	for i := range claims {
		if claims[i].matcher != nil {
			continue
		}

		if len(claims[i].Pattern) > maxPatternLength {
			return errPatternTooLong
		}

		if claims[i].Type == "" {
			if !deprecated {
				return errStringClaimRequiresCompat
			}

			if err := resolveDeprecatedStringClaim(&claims[i]); err != nil {
				return err
			}

			continue
		}

		rm, ok := tss.matchers[strings.ToLower(claims[i].Type)]
		if !ok {
			return ErrUnsupportedMatcherType
		}

		if err := validatePattern(rm.matcher, claims[i].Pattern); err != nil {
			return fmt.Errorf("invalid %s pattern in JWT claim: %w", rm.canonicalName, err)
		}

		// Canonicalise the type so it matches the casing the hub uses
		// everywhere else (subscription events, subscription API, …).
		claims[i].Type = rm.canonicalName
		claims[i].matcher = rm.matcher
	}

	return nil
}

const (
	// exactMatcherTypeName is the name of the built-in exact matcher type.
	exactMatcherTypeName = "exact"

	// deprecatedMatcherTypeName tags topic matchers created from the v8
	// `topic=` query parameter or bare-string JWT claims. The underscore
	// prefix keeps the name out of the public registry namespace (operators
	// can't register it via WithMatcherType). The string leaks into debug
	// log attributes, so pick something that reads usefully there.
	deprecatedMatcherTypeName = "_deprecated_topic"
)
