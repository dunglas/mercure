//go:build deprecated_transport

package caddy

import (
	"strings"
	"testing"

	"github.com/caddyserver/caddy/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeprecatedTransportDefaultHubName(t *testing.T) {
	t.Parallel()

	unnamed := &Mercure{TransportURL: "local://"}
	namedDefault := &Mercure{Name: "default", TransportURL: "local://"}
	other := &Mercure{Name: "other", TransportURL: "local://"}

	assert.Equal(t, unnamed.deprecatedTransportKey(), namedDefault.deprecatedTransportKey())
	assert.NotEqual(t, unnamed.deprecatedTransportKey(), other.deprecatedTransportKey())
}

func TestUnnamedHubFailedReloadPreservesDeprecatedTransport(t *testing.T) {
	t.Cleanup(func() { assert.NoError(t, caddy.Stop()) })

	config := func(names ...string) []byte {
		return []byte(strings.ReplaceAll(string(hubNamesConfig(names)), `"transport":{"name":"local"}`, `"transport_url":"local://"`))
	}

	require.NoError(t, caddy.Load(config(""), true))
	require.ErrorContains(t, caddy.Load(config("", ""), true), errMultipleUnnamedHubs.Error())

	refs, exists := transports.References("default\x00local://")
	assert.True(t, exists)
	assert.Equal(t, 1, refs, "a failed reload must retain the active transport")
	assert.Equal(t, 1, unnamedHubCount())
}
