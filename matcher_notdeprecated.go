//go:build !deprecated_topic

package mercure

// resolveDeprecatedStringClaim is the stub compiled without the
// deprecated_topic build tag: bare-string claims are refused even when the
// operator enabled WithProtocolVersionCompatibility, because the
// deprecatedMatcher implementation is not in the binary.
func resolveDeprecatedStringClaim(*matcherClaim) error {
	return errStringClaimRequiresCompat
}
