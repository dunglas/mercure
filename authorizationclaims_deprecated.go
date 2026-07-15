//go:build deprecated_claim

package mercure

// deprecatedMercureClaims carries the bespoke mercure JWT claim, removed from
// the modern protocol in favor of the RFC 9396 authorization_details claim.
// It is embedded in claims only in deprecated_claim builds and honored only in
// compatibility mode.
type deprecatedMercureClaims struct {
	// Mercure is the legacy claim.
	//
	// Deprecated: use authorization_details instead.
	Mercure mercureClaim `json:"mercure"`
	// MercureNamespaced is the legacy namespaced fallback claim.
	//
	// Deprecated: use authorization_details instead.
	MercureNamespaced *mercureClaim `json:"https://mercure.rocks/"`
}

type mercureClaim struct {
	Publish   []matcherClaim `json:"publish"`
	Subscribe []matcherClaim `json:"subscribe"`
	Payload   any            `json:"payload"`
}
