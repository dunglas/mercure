//go:build deprecated_topic

package mercure

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveMatcherClaimsDeprecated(t *testing.T) {
	t.Parallel()

	claims := []matcherClaim{
		{topicMatcher: topicMatcher{Pattern: "foo"}},                // Unresolved string
		{topicMatcher: topicMatcher{Pattern: "bar", Type: "exact"}}, // Explicit Exact
	}

	require.NoError(t, resolveMatcherClaims(newExactStore(t), claims, true))

	assert.Equal(t, deprecatedMatcherTypeName, claims[0].Type)
	assert.Equal(t, deprecatedMatcher, claims[0].matcher)

	assert.Equal(t, "Exact", claims[1].Type)
	assert.Equal(t, ExactMatcher, claims[1].matcher)
}
