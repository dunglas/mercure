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
// parameter carrying the access token, honored only in compatibility mode. The
// modern parameter is "access_token".
const legacyAuthorizationParam = "authorization"

// compatClaimsEnabled reports whether legacy mercure-claim behavior is active:
// the code is compiled in and the operator enabled compatibility mode.
func (h *Hub) compatClaimsEnabled() bool {
	return h.isBackwardCompatiblyEnabledWith(8)
}

// requireATJWT reports whether access tokens must carry the at+jwt typ header
// and a matching audience. Relaxed in compatibility mode so legacy tokens
// (which predate RFC 9068) keep working.
func (h *Hub) requireATJWT() bool {
	return !h.compatClaimsEnabled()
}

// legacyAuthQueryParam returns the token carried by the deprecated
// "authorization" query parameter when compatibility mode is enabled. The
// modern parameter is "access_token".
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

// readCookie returns the authorization cookie. In compatibility mode, the
// pre-1.0 cookie name is accepted as a fallback when the configured name is
// absent, so subscribers still sending "mercureAuthorization" keep working.
func (h *Hub) readCookie(r *http.Request) (*http.Cookie, error) {
	cookie, err := r.Cookie(h.cookieName)
	if err == nil || !h.compatClaimsEnabled() {
		return cookie, err //nolint:wrapcheck
	}

	return r.Cookie(legacyCookieName) //nolint:wrapcheck
}

// resolveLegacyClaims converts the legacy mercure claim into the validated
// authorization details shape, so the grant logic is uniform across token
// formats. It runs only in compatibility mode.
func (h *Hub) resolveLegacyClaims(c *claims) error {
	if !h.compatClaimsEnabled() {
		return nil
	}

	mc := c.Mercure
	if c.MercureNamespaced != nil {
		mc = *c.MercureNamespaced
	}

	if len(mc.Publish) > maxClaimMatchers || len(mc.Subscribe) > maxClaimMatchers {
		return ErrTooManyClaimMatchers
	}

	// Bare-string v8 claims are only meaningful when the deprecated_topic
	// matcher code is compiled in: allowsAlternateTopics() is true only then.
	// Using isBackwardCompatiblyEnabledWith(8) here would be always-true (this
	// function already returned unless compat is on), so a "*" string claim
	// would authorize every topic via the wildcard short-circuit even in a
	// deprecated_claim-only build where the v8 matcher is absent.
	deprecated := h.allowsAlternateTopics()
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
	authz := &mercureAuthz{}

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
