//go:build !deprecated_claim

package mercure

import "net/http"

// compatClaimsEnabled reports whether legacy mercure-claim behavior is active.
// It never is without the deprecated_claim build tag.
func (*opt) compatClaimsEnabled() bool {
	return false
}

// legacyAuthQueryParam is the stub for the deprecated "authorization" query
// parameter: it is never honored without the deprecated_claim tag.
func (h *Hub) legacyAuthQueryParam(*http.Request) (string, bool) {
	return "", false
}

// readCookie returns the authorization cookie. Without the deprecated_claim
// tag, only the configured cookie name is accepted.
func (h *Hub) readCookie(r *http.Request) (*http.Cookie, bool, error) {
	cookie, err := r.Cookie(h.cookieName)

	return cookie, false, err //nolint:wrapcheck
}

// This build never unmarshals legacy claims.
func dropLegacyClaims(*claims) {}

// resolveLegacyClaims is never reached without the deprecated_claim tag: no
// token is legacy.
func (h *Hub) resolveLegacyClaims(*claims) error {
	return nil
}

// legacyPayloadFallback returns no fallback payload without the
// deprecated_claim tag.
func (s *Subscriber) legacyPayloadFallback() any {
	return nil
}
