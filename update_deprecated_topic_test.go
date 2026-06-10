//go:build deprecated_topic

package mercure

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpdateJSONAlternateTopics checks that v8 alternate topics survive a
// JSON round trip (bolt history) in builds with the deprecated_topic tag.
func TestUpdateJSONAlternateTopics(t *testing.T) {
	t.Parallel()

	u := testUpdate(&Update{}, "https://example.com/a", "https://example.com/b")

	out, err := json.Marshal(u)
	require.NoError(t, err)
	assert.JSONEq(t, `{"Data":"","ID":"","Type":"","Retry":0,"Topics":["https://example.com/a","https://example.com/b"],"Private":false,"Debug":false}`, string(out))

	var decoded *Update

	require.NoError(t, json.Unmarshal(out, &decoded))
	assert.Equal(t, "https://example.com/a", decoded.Topic)
	assert.Equal(t, []string{"https://example.com/b"}, decoded.Topics)
	assert.Equal(t, u, decoded)
}

// TestUpdateLegacyTopicsOnly checks that updates built by v8-era Go code that
// only sets the deprecated Topics field still dispatch correctly.
func TestUpdateLegacyTopicsOnly(t *testing.T) {
	t.Parallel()

	u := &Update{}
	u.Topics = []string{"https://example.com/a", "https://example.com/b"}

	assert.Equal(t, []string{"https://example.com/a", "https://example.com/b"}, u.topics())
}
