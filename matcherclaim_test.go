//go:build deprecated_claim

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

// TestMatcherClaimRejectsAmbiguousJSON mirrors the authorization_details
// hardening for the deprecated mercure claim: a duplicate object member or
// invalid UTF-8 makes the entry mean different things to different parsers,
// so the claim is rejected instead of resolved to one of the readings.
func TestMatcherClaimRejectsAmbiguousJSON(t *testing.T) {
	t.Parallel()

	for name, claim := range map[string]string{
		"duplicate match":      `{"match": "a", "match": "b"}`,
		"duplicate match_type": `{"match": "a", "match_type": "exact", "match_type": "urlpattern"}`,
		"duplicate payload":    `{"match": "a", "payload": 1, "payload": 2}`,
		"invalid UTF-8 object": "{\"match\": \"a\xffb\"}",
		"invalid UTF-8 string": "\"a\xffb\"",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var mc matcherClaim
			require.Error(t, json.Unmarshal([]byte(claim), &mc))
		})
	}
}

func TestMatcherClaimMarshalRoundTrip(t *testing.T) {
	t.Parallel()

	// Object form
	in := matcherClaim{Type: MatcherTypeURLPattern, Pattern: "https://example.com/:id", Payload: map[string]any{"a": "b"}}
	b, err := json.Marshal(&in)
	require.NoError(t, err)
	assert.JSONEq(t, `{"match": "https://example.com/:id", "match_type": "urlpattern", "payload": {"a": "b"}}`, string(b))

	// String form (unresolved)
	in = matcherClaim{Pattern: "https://example.com/foo"}
	b, err = json.Marshal(&in)
	require.NoError(t, err)
	assert.JSONEq(t, `"https://example.com/foo"`, string(b))
}

func TestResolveMatcherClaims(t *testing.T) {
	t.Parallel()

	tms, err := NewTopicMatcherStore(0)
	require.NoError(t, err)

	// Object-form claims with valid types resolve in modern mode.
	cs := []matcherClaim{
		{Type: MatcherTypeExact, Pattern: "foo"},
		{Type: MatcherTypeURLPattern, Pattern: "https://example.com/:id"},
	}
	require.NoError(t, resolveMatcherClaims(tms, cs, false))

	// Bare-string claims are rejected in modern mode.
	cs = []matcherClaim{{Pattern: "foo"}}
	require.ErrorIs(t, resolveMatcherClaims(tms, cs, false), errStringClaimRequiresCompat)

	// Unknown matcher types are rejected; type values are case-sensitive.
	cs = []matcherClaim{{Type: "URLPattern", Pattern: "foo"}}
	require.ErrorIs(t, resolveMatcherClaims(tms, cs, false), ErrUnsupportedMatcherType)

	// Forged internal type is rejected in modern mode.
	cs = []matcherClaim{{Type: deprecatedMatcherTypeName, Pattern: "foo"}}
	require.ErrorIs(t, resolveMatcherClaims(tms, cs, false), errStringClaimRequiresCompat)

	// Invalid URLPattern pattern is rejected.
	cs = []matcherClaim{{Type: MatcherTypeURLPattern, Pattern: "{unclosed"}}
	assert.Error(t, resolveMatcherClaims(tms, cs, false))
}
