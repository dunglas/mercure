//go:build !deprecated_claim

package mercure

import "net/http"

// compatClaimsEnabled reports whether legacy mercure-claim behavior is active.
// It never is without the deprecated_claim build tag.
func (h *Hub) compatClaimsEnabled() bool {
	return false
}

// requireATJWT reports whether access tokens must carry the at+jwt typ header
// and a matching audience. Always required without the deprecated_claim tag.
func (h *Hub) requireATJWT() bool {
	return true
}

// legacyAuthQueryParam is the stub for the deprecated "authorization" query
// parameter: it is never honored without the deprecated_claim tag.
func (h *Hub) legacyAuthQueryParam(*http.Request) (string, bool) {
	return "", false
}

// resolveLegacyClaims is a no-op without the deprecated_claim tag: the legacy
// mercure claim grants nothing.
func (h *Hub) resolveLegacyClaims(*claims) error {
	return nil
}

// legacyPayloadFallback returns no fallback payload without the
// deprecated_claim tag.
func (s *Subscriber) legacyPayloadFallback() any {
	return nil
}
