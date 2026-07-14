package mercure

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatcherClaimUnmarshalString(t *testing.T) {
	t.Parallel()

	var mc matcherClaim
	require.NoError(t, json.Unmarshal([]byte(`"https://example.com/foo"`), &mc))

	assert.Equal(t, "https://example.com/foo", mc.Pattern)
	assert.Empty(t, mc.Type) // Unresolved — resolved later based on protocol version
	assert.Nil(t, mc.Payload)
}

func TestMatcherClaimUnmarshalObject(t *testing.T) {
	t.Parallel()

	var mc matcherClaim
	require.NoError(t, json.Unmarshal([]byte(`{"match": "https://example.com/:id", "match_type": "urlpattern"}`), &mc))

	assert.Equal(t, "https://example.com/:id", mc.Pattern)
	assert.Equal(t, MatcherTypeURLPattern, mc.Type)
	assert.Nil(t, mc.Payload)
}

func TestMatcherClaimUnmarshalObjectDefaultsToExact(t *testing.T) {
	t.Parallel()

	var mc matcherClaim
	require.NoError(t, json.Unmarshal([]byte(`{"match": "https://example.com/foo"}`), &mc))

	assert.Equal(t, MatcherTypeExact, mc.Type)
}

func TestMatcherClaimUnmarshalObjectWithPayload(t *testing.T) {
	t.Parallel()

	var mc matcherClaim
	require.NoError(t, json.Unmarshal([]byte(`{"match": "https://example.com/:id", "match_type": "urlpattern", "payload": {"user": "alice"}}`), &mc))

	payloadMap, ok := mc.Payload.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "alice", payloadMap["user"])
}

// TestMatcherClaimUnmarshalReset guards against state leaking between decode
// calls when a matcherClaim is reused.
func TestMatcherClaimUnmarshalReset(t *testing.T) {
	t.Parallel()

	var mc matcherClaim
	require.NoError(t, json.Unmarshal([]byte(`{"match": "a", "match_type": "urlpattern", "payload": 1}`), &mc))
	require.NoError(t, json.Unmarshal([]byte(`"b"`), &mc))

	assert.Equal(t, "b", mc.Pattern)
	assert.Empty(t, mc.Type)
	assert.Nil(t, mc.Payload)
}

func TestMatcherClaimMarshalRoundTrip(t *testing.T) {
	t.Parallel()

	// Object form
	in := matcherClaim{TopicMatcher: TopicMatcher{Type: MatcherTypeURLPattern, Pattern: "https://example.com/:id"}, Payload: map[string]any{"a": "b"}}
	b, err := json.Marshal(&in)
	require.NoError(t, err)
	assert.JSONEq(t, `{"match": "https://example.com/:id", "match_type": "urlpattern", "payload": {"a": "b"}}`, string(b))

	// String form (unresolved)
	in = matcherClaim{TopicMatcher: TopicMatcher{Pattern: "https://example.com/foo"}}
	b, err = json.Marshal(&in)
	require.NoError(t, err)
	assert.JSONEq(t, `"https://example.com/foo"`, string(b))
}

func TestResolveMatcherClaims(t *testing.T) {
	t.Parallel()

	tss, err := NewTopicSelectorStore(0)
	require.NoError(t, err)

	// Object-form claims with valid types resolve in modern mode.
	cs := []matcherClaim{
		{TopicMatcher: TopicMatcher{Type: MatcherTypeExact, Pattern: "foo"}},
		{TopicMatcher: TopicMatcher{Type: MatcherTypeURLPattern, Pattern: "https://example.com/:id"}},
	}
	require.NoError(t, resolveMatcherClaims(tss, cs, false))

	// Bare-string claims are rejected in modern mode.
	cs = []matcherClaim{{TopicMatcher: TopicMatcher{Pattern: "foo"}}}
	require.ErrorIs(t, resolveMatcherClaims(tss, cs, false), errStringClaimRequiresCompat)

	// Unknown matcher types are rejected; type values are case-sensitive.
	cs = []matcherClaim{{TopicMatcher: TopicMatcher{Type: "URLPattern", Pattern: "foo"}}}
	require.ErrorIs(t, resolveMatcherClaims(tss, cs, false), ErrUnsupportedMatcherType)

	// Forged internal type is rejected in modern mode.
	cs = []matcherClaim{{TopicMatcher: TopicMatcher{Type: deprecatedMatcherTypeName, Pattern: "foo"}}}
	require.ErrorIs(t, resolveMatcherClaims(tss, cs, false), errStringClaimRequiresCompat)

	// Invalid URLPattern pattern is rejected.
	cs = []matcherClaim{{TopicMatcher: TopicMatcher{Type: MatcherTypeURLPattern, Pattern: "{unclosed"}}}
	assert.Error(t, resolveMatcherClaims(tss, cs, false))
}
