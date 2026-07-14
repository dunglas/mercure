package mercure

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
)

// errStringClaimRequiresCompat is returned when a JWT mercure.publish /
// mercure.subscribe claim uses the deprecated string form in modern mode. The
// protocol requires the object form; the string form is accepted only under
// WithProtocolVersionCompatibility. Mapped to 401 on the wire.
var errStringClaimRequiresCompat = errors.New("string-form matcher claims require backward-compatibility mode")

// errMissingMatchProperty is returned when an object-form matcher claim omits
// the required "match" property (or is JSON null). The protocol requires every
// topic matcher object to carry a "match" property; a token containing such an
// entry must be rejected rather than silently treated as an empty pattern.
var errMissingMatchProperty = errors.New(`topic matcher object is missing the required "match" property`)

// matcherClaim represents a single entry in the mercure.publish or
// mercure.subscribe JWT claim. It supports both the deprecated string format
// and the object format.
type matcherClaim struct {
	TopicMatcher

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
		Match     string      `json:"match"`
		MatchType MatcherType `json:"match_type,omitempty"`
		Payload   any         `json:"payload,omitempty"`
	}{mc.Pattern, mc.Type, mc.Payload}

	b, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal matcher claim object: %w", err)
	}

	return b, nil
}

// UnmarshalJSON handles both string and object formats in JWT claims.
// String: v8 form, accepted only in backward-compatibility mode.
// Object: {"match": "pattern", "match_type": "exact", "payload": {...}};
// match_type is case-sensitive and defaults to Exact.
//
// Always resets every field of the receiver before populating it, so reusing
// a matcherClaim across decode calls does not leak the previous Type/Payload.
func (mc *matcherClaim) UnmarshalJSON(data []byte) error {
	*mc = matcherClaim{}

	// A null entry is neither a v8 string selector nor a valid matcher object;
	// json.Unmarshal(null) is a silent no-op, so reject it explicitly rather
	// than accept an empty-pattern matcher.
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return errMissingMatchProperty
	}

	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		// Empty Type signals "unresolved string claim"; resolveMatcherClaims
		// decides what it means based on the protocol version.
		mc.Pattern = s

		return nil
	}

	// Match is a pointer so an absent property is distinguishable from an
	// explicit empty string: the protocol requires the property to be present.
	var obj struct {
		Match     *string     `json:"match"`
		MatchType MatcherType `json:"match_type"`
		Payload   any         `json:"payload"`
	}

	if err := json.Unmarshal(data, &obj); err != nil {
		return err //nolint:wrapcheck
	}

	if obj.Match == nil {
		return errMissingMatchProperty
	}

	mc.Pattern = *obj.Match
	mc.Payload = obj.Payload
	mc.Type = obj.MatchType

	if mc.Type == "" {
		mc.Type = MatcherTypeExact
	}

	return nil
}

// resolveMatcherClaims validates the matcher type and pattern of each claim.
//
// String-form entries (Type == "") are only permitted under deprecated mode,
// where they map to the v8 "exact OR URI Template" rule. In modern mode the
// protocol requires the object form; silently reinterpreting bare strings as
// Exact would change the meaning of tokens minted for v8.
//
// JWT claims are untrusted until the signature is checked and stay
// attacker-shaped afterwards, so the same maxPatternLength cap and
// control-character rejection the query parser enforces also apply here (the
// entry count is already capped by validateJWT).
func resolveMatcherClaims(tss *TopicSelectorStore, claims []matcherClaim, deprecated bool) error {
	for i := range claims {
		if len(claims[i].Pattern) > maxPatternLength {
			return errPatternTooLong
		}

		if !validProtocolString(claims[i].Pattern) {
			return errInvalidMatcherValue
		}

		switch claims[i].Type {
		case "":
			if !deprecated {
				return errStringClaimRequiresCompat
			}

			claims[i].Type = deprecatedMatcherTypeName
		case deprecatedMatcherTypeName:
			// Only reachable through a forged object-form claim or a re-run
			// of an already-resolved string claim; reject it in modern mode.
			if !deprecated {
				return errStringClaimRequiresCompat
			}
		case MatcherTypeExact, MatcherTypeURLPattern:
			if err := tss.validatePattern(claims[i].TopicMatcher); err != nil {
				return fmt.Errorf("invalid matcher in JWT claim: %w", err)
			}
		default:
			return ErrUnsupportedMatcherType
		}
	}

	return nil
}

// writeMatcherClaimError translates a resolveMatcherClaims error into an HTTP
// response. Every failure here is a defect of the matcher entries carried by
// the presented token (unknown matcher type, invalid pattern, control
// characters, string form without compatibility mode), so the protocol
// classifies them all as invalid_token (401), never invalid_request (400). It
// also logs the cause at info level so operators upgrading from v8 see a hint
// without having to enable debug logging.
func writeMatcherClaimError(ctx context.Context, logger *slog.Logger, w http.ResponseWriter, err error) {
	http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)

	if logger == nil || !logger.Enabled(ctx, slog.LevelInfo) {
		return
	}

	switch {
	case errors.Is(err, errStringClaimRequiresCompat):
		logger.LogAttrs(ctx, slog.LevelInfo,
			`JWT contains v8 bare-string topic claims. Re-mint tokens with the {"match": "...", "match_type": "..."} object form, or run the hub with WithProtocolVersionCompatibility(8) and the deprecated_topic build tag to keep accepting them.`,
			slog.Any("error", err))
	case errors.Is(err, ErrUnsupportedMatcherType):
		logger.LogAttrs(ctx, slog.LevelInfo,
			`JWT references an unknown matcher type; supported values are "exact" and "urlpattern" (case-sensitive).`,
			slog.Any("error", err))
	default:
		logger.LogAttrs(ctx, slog.LevelInfo,
			"Failed to resolve JWT topic matcher claims",
			slog.Any("error", err))
	}
}
