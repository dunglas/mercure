//go:build !deprecated_topic

package mercure

import "fmt"

// resolveDeprecatedStringClaim is the stub compiled without the
// deprecated_topic build tag: bare-string claims are refused even when the
// operator enabled WithProtocolVersionCompatibility, because the
// deprecatedMatcher implementation is not in the binary.
func resolveDeprecatedStringClaim(*matcherClaim) error {
	return errStringClaimRequiresCompat
}

// bindDeprecatedMatcher is the stub compiled without the deprecated_topic
// build tag: a deserialized subscriber carrying the deprecated sentinel
// cannot be revived because the matcher implementation is not in the binary.
func bindDeprecatedMatcher(m *topicMatcher) error {
	return fmt.Errorf("%w: %s (rebuild with the deprecated_topic build tag)", ErrUnsupportedMatcherType, m.Type)
}
