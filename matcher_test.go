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
	require.NoError(t, json.Unmarshal([]byte(`{"match": "https://example.com/:id", "matchType": "URLPattern"}`), &mc))

	assert.Equal(t, "https://example.com/:id", mc.Pattern)
	assert.Equal(t, "urlpattern", mc.Type)
	assert.Nil(t, mc.Payload)
}

func TestMatcherClaimUnmarshalObjectWithPayload(t *testing.T) {
	t.Parallel()

	var mc matcherClaim
	require.NoError(t, json.Unmarshal([]byte(`{"match": ".*", "matchType": "Regexp", "payload": {"user": "alice"}}`), &mc))

	assert.Equal(t, ".*", mc.Pattern)
	assert.Equal(t, "regexp", mc.Type)
	assert.NotNil(t, mc.Payload)

	payloadMap, ok := mc.Payload.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "alice", payloadMap["user"])
}

func TestMatcherClaimUnmarshalObjectDefaultType(t *testing.T) {
	t.Parallel()

	// When matchType is omitted, it defaults to "exact"
	var mc matcherClaim
	require.NoError(t, json.Unmarshal([]byte(`{"match": "foo"}`), &mc))

	assert.Equal(t, "foo", mc.Pattern)
	assert.Equal(t, "exact", mc.Type)
}

func TestMatcherClaimUnmarshalArray(t *testing.T) {
	t.Parallel()

	// Test unmarshaling an array of mixed string and object claims
	data := `[
		"https://example.com/foo",
		{"match": "https://example.com/:id", "matchType": "URLPattern"},
		{"match": ".*", "matchType": "Regexp", "payload": {"key": "val"}},
		"bar"
	]`

	var claims []matcherClaim
	require.NoError(t, json.Unmarshal([]byte(data), &claims))

	require.Len(t, claims, 4)

	// String claims
	assert.Equal(t, "https://example.com/foo", claims[0].Pattern)
	assert.Empty(t, claims[0].Type)
	assert.Equal(t, "bar", claims[3].Pattern)
	assert.Empty(t, claims[3].Type)

	// Object claims
	assert.Equal(t, "https://example.com/:id", claims[1].Pattern)
	assert.Equal(t, "urlpattern", claims[1].Type)
	assert.Equal(t, ".*", claims[2].Pattern)
	assert.Equal(t, "regexp", claims[2].Type)
	assert.NotNil(t, claims[2].Payload)
}

func TestMatcherClaimUnmarshalInvalid(t *testing.T) {
	t.Parallel()

	var mc matcherClaim
	// Arrays and booleans are not valid claim entries
	require.Error(t, json.Unmarshal([]byte(`[1,2,3]`), &mc))
	require.Error(t, json.Unmarshal([]byte(`true`), &mc))
}

func newExactStore(t *testing.T) *TopicSelectorStore {
	t.Helper()

	tss, err := NewTopicSelectorStore(0)
	require.NoError(t, err)
	tss.RegisterMatcherType("Exact", ExactMatcher)

	return tss
}

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

func TestResolveMatcherClaimsModernRejectsStringForm(t *testing.T) {
	t.Parallel()

	claims := []matcherClaim{{topicMatcher: topicMatcher{Pattern: "foo"}}}

	err := resolveMatcherClaims(newExactStore(t), claims, false)
	assert.ErrorIs(t, err, errStringClaimRequiresCompat)
}

func TestResolveMatcherClaimsModernAcceptsObjectForm(t *testing.T) {
	t.Parallel()

	claims := []matcherClaim{{topicMatcher: topicMatcher{Pattern: "foo", Type: "exact"}}}

	require.NoError(t, resolveMatcherClaims(newExactStore(t), claims, false))
	assert.Equal(t, "Exact", claims[0].Type)
	assert.Equal(t, ExactMatcher, claims[0].matcher)
}

func TestResolveMatcherClaimsUnsupportedType(t *testing.T) {
	t.Parallel()

	claims := []matcherClaim{{topicMatcher: topicMatcher{Pattern: "foo", Type: "UnknownType"}}}

	err := resolveMatcherClaims(newExactStore(t), claims, false)
	assert.ErrorIs(t, err, ErrUnsupportedMatcherType)
}
