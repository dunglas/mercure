//go:build !deprecated_topic

package mercure

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchDeprecatedStub(t *testing.T) {
	t.Parallel()

	tss, err := NewTopicSelectorStore(0)
	require.NoError(t, err)

	// Without the deprecated_topic build tag, v8 matchers never match.
	m := topicMatcher{Type: deprecatedMatcherTypeName, Pattern: "foo"}
	assert.False(t, tss.matchMatcher([]string{"foo"}, m))
}
