//go:build deprecated_claim && !deprecated_topic

package mercure

// The deprecated_claim tag enables the legacy "mercure" JWT claim, whose
// string-form topic selectors are evaluated by the v8 matcher that is compiled
// in only under the deprecated_topic tag. Building deprecated_claim without
// deprecated_topic would let a "*" string claim authorize every topic (via the
// wildcard short-circuit) while every non-wildcard string claim silently
// matches nothing — an authorization hazard — so the combination is rejected at
// build time. Always build the two tags together.
//
// The reference below is intentionally undefined so this configuration fails to
// compile with a message naming the required tag.
var _ = deprecatedClaimRequiresDeprecatedTopicTag
