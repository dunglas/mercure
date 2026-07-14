//go:build deprecated_claim

package mercure

import (
	"log/slog"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSubscriptionPayloadFallbackToGlobal verifies the fallback to the legacy
// mercure.payload when no per-detail payload matches the subscription's
// matcher. The global payload fallback is a deprecated_claim feature.
func TestSubscriptionPayloadFallbackToGlobal(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)
	logger := slog.Default()

	sub := NewLocalSubscriber("", logger, hub.topicSelectorStore)
	matchers, err := hub.parseMatchers(url.Values{
		"match": {"https://example.com/foo"},
	}, false)
	require.NoError(t, err)

	mc := mercureClaim{
		Payload: map[string]any{"global": true},
		Subscribe: []matcherClaim{
			// A claim that doesn't match the subscription's matcher.
			{TopicMatcher: TopicMatcher{Type: MatcherTypeExact, Pattern: "https://other.example.com/x"}, Payload: map[string]any{"tag": "ignored"}},
		},
	}
	sub.Claims = &claims{
		deprecatedMercureClaims: deprecatedMercureClaims{Mercure: mc},
		authz:                   mercureAuthzFromLegacy(mc),
	}

	sub.setMatchers(matchers, nil)

	subs := sub.getSubscriptions(subscriptionFilter{}, "", true)
	require.Len(t, subs, 1)

	p, ok := subs[0].Payload.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, p["global"])
}
