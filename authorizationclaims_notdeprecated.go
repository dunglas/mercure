//go:build !deprecated_claim

package mercure

// deprecatedMercureClaims is empty without the deprecated_claim build tag: the
// bespoke mercure JWT claim is not part of the modern protocol.
type deprecatedMercureClaims struct{} //nolint:unused // embedded in claims for build-mode symmetry, like deprecatedTopics in Update
