//go:build deprecated_topic

package mercure

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResolveMatcherClaimsDeprecated verifies that bare-string claims bind to
// the v8 matcher in compatibility mode and stay idempotent across re-runs.
func TestResolveMatcherClaimsDeprecated(t *testing.T) {
	t.Parallel()

	tss, err := NewTopicSelectorStore(0)
	require.NoError(t, err)

	cs := []matcherClaim{
		{topicMatcher: topicMatcher{Pattern: "https://example.com/{id}"}},
		{topicMatcher: topicMatcher{Type: MatcherTypeExact, Pattern: "foo"}},
	}
	require.NoError(t, resolveMatcherClaims(tss, cs, true))
	assert.Equal(t, deprecatedMatcherTypeName, cs[0].Type)
	assert.Equal(t, MatcherTypeExact, cs[1].Type)

	// Idempotent: resolving again keeps the deprecated binding.
	require.NoError(t, resolveMatcherClaims(tss, cs, true))
	assert.Equal(t, deprecatedMatcherTypeName, cs[0].Type)

	// The resolved claim matches via the v8 rules.
	assert.True(t, tss.matchMatcher([]string{"https://example.com/42"}, cs[0].topicMatcher))
}
