//go:build deprecated_transport

package caddy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeprecatedTransportDefaultHubName(t *testing.T) {
	t.Parallel()

	unnamed := &Mercure{TransportURL: "local://"}
	namedDefault := &Mercure{Name: "default", TransportURL: "local://"}
	other := &Mercure{Name: "other", TransportURL: "local://"}

	assert.Equal(t, unnamed.deprecatedTransportKey(), namedDefault.deprecatedTransportKey())
	assert.NotEqual(t, unnamed.deprecatedTransportKey(), other.deprecatedTransportKey())
}
