//go:build deprecated_claim

package mercure

import (
	"errors"
	"net/http"
)

// ErrTooManyClaimMatchers is returned when the legacy mercure.subscribe or
// mercure.publish claim exceeds maxClaimMatchers.
var ErrTooManyClaimMatchers = errors.New("too many matchers in mercure claim")

// legacyCookieName is the pre-1.0 authorization cookie name, accepted as a
// fallback only in compatibility mode. The modern name is defaultCookieName.
const legacyCookieName = "mercureAuthorization"

// legacyAuthorizationParam is the deprecated "authorization" URI query
// parameter carrying the access token, honored only in compatibility mode.
// Modern mode accepts no query parameter (RFC 9700 §4.3.2).
const legacyAuthorizationParam = "authorization"

// compatClaimsEnabled reports whether legacy mercure-claim behavior is active:
// the code is compiled in and the operator enabled compatibility mode.
func (o *opt) compatClaimsEnabled() bool {
	return o.isBackwardCompatiblyEnabledWith(8)
}

// legacyAuthQueryParam returns the token carried by the deprecated
// "authorization" query parameter when compatibility mode is enabled. Modern
// mode accepts no query parameter (RFC 9700 §4.3.2).
func (h *Hub) legacyAuthQueryParam(r *http.Request) (string, bool) {
	if !h.compatClaimsEnabled() {
		return "", false
	}

	q, ok := r.URL.Query()[legacyAuthorizationParam]
	if !ok || len(q) != 1 || len(q[0]) < minCompactJWSLen {
		return "", false
	}

	return q[0], true
}

// readCookie returns the authorization cookie, and whether it is the pre-1.0
// one. In compatibility mode, that name is accepted as a fallback when the
// configured name is absent, so subscribers still sending
// "mercureAuthorization" keep working.
func (h *Hub) readCookie(r *http.Request) (*http.Cookie, bool, error) {
	cookie, err := r.Cookie(h.cookieName)
	if err == nil || !h.compatClaimsEnabled() {
		return cookie, false, err //nolint:wrapcheck
	}

	cookie, err = r.Cookie(legacyCookieName)

	return cookie, true, err //nolint:wrapcheck
}

// Discard ignored legacy payloads so subscription events cannot expose them.
func dropLegacyClaims(c *claims) {
	c.Mercure = mercureClaim{}
	c.MercureNamespaced = nil
}

// resolveLegacyClaims converts the legacy mercure claim of a 0.x token into
// the validated authorization details shape, so the grant logic is uniform
// across token formats.
func (h *Hub) resolveLegacyClaims(c *claims) error {
	mc := c.Mercure
	if c.MercureNamespaced != nil {
		mc = *c.MercureNamespaced
	}

	if len(mc.Publish) > maxClaimMatchers || len(mc.Subscribe) > maxClaimMatchers {
		return ErrTooManyClaimMatchers
	}

	// Bare-string v8 claims are only meaningful when the deprecated_topic
	// matcher code is compiled in. Using isBackwardCompatiblyEnabledWith(8)
	// here would be always-true (callers only pass 0.x tokens, accepted only
	// in compatibility mode), so a "*" string claim would authorize every
	// topic via the wildcard short-circuit even in a deprecated_claim-only
	// build where the v8 matcher is absent.
	deprecated := deprecatedMatcherTypeCompiled()
	if err := resolveMatcherClaims(h.topicMatcherStore, mc.Publish, deprecated); err != nil {
		return err
	}

	if err := resolveMatcherClaims(h.topicMatcherStore, mc.Subscribe, deprecated); err != nil {
		return err
	}

	c.Mercure = mc
	c.authz = mercureAuthzFromLegacy(mc)

	return nil
}

// mercureAuthzFromLegacy builds a mercureAuthz from a resolved legacy claim,
// one subscribe detail per claim so per-claim payloads are preserved.
func mercureAuthzFromLegacy(mc mercureClaim) *mercureAuthz {
	// A JSON [] decodes to a non-nil slice: an empty claim is distinct from a missing one.
	authz := &mercureAuthz{legacyPublish: mc.Publish != nil}

	if len(mc.Publish) > 0 {
		authz.details = append(authz.details, validatedDetail{publish: true, topics: matcherClaimTopics(mc.Publish)})
	}

	for _, sc := range mc.Subscribe {
		authz.details = append(authz.details, validatedDetail{
			subscribe: true,
			topics:    []TopicMatcher{sc.TopicMatcher},
			payload:   sc.Payload,
		})
	}

	return authz
}

func matcherClaimTopics(claims []matcherClaim) []TopicMatcher {
	topics := make([]TopicMatcher, len(claims))
	for i := range claims {
		topics[i] = claims[i].TopicMatcher
	}

	return topics
}

// legacyPayloadFallback returns the global mercure.payload, used when no
// per-subscription payload matched.
func (s *Subscriber) legacyPayloadFallback() any {
	if s.Claims == nil {
		return nil
	}

	return s.Claims.Mercure.Payload
}
