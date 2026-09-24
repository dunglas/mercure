package caddy

import (
	"testing"

	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/dunglas/mercure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBoltCleanupFrequency(t *testing.T) {
	t.Parallel()

	for block, want := range map[string]float64{
		"bolt {\n\tsize 5\n}":                          mercure.BoltDefaultCleanupFrequency,
		"bolt {\n\tsize 5\n\tcleanup_frequency 0\n}":   0,
		"bolt {\n\tsize 5\n\tcleanup_frequency 0.2\n}": 0.2,
	} {
		var b Bolt
		require.NoError(t, b.UnmarshalCaddyfile(caddyfile.NewTestDispenser(block)))
		assert.InDelta(t, want, b.cleanupFrequency(), 0)
	}
}
