package caddy

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/caddyserver/caddy/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func hubContext(t *testing.T, name string) caddy.Context {
	t.Helper()

	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	t.Cleanup(cancel)

	ctx = ctx.WithValue(HubNameContextKey, name)

	return ctx.WithValue(SubscriberListCacheSizeContextKey, 0)
}

func TestLocalTransportScopedByHub(t *testing.T) {
	provision := func(name string) *Local {
		l := &Local{}
		require.NoError(t, l.Provision(hubContext(t, name)))
		t.Cleanup(func() { assert.NoError(t, l.Cleanup()) })

		return l
	}

	a, b, a2 := provision("a"), provision("b"), provision("a")
	assert.NotSame(t, a.transport, b.transport)
	assert.Same(t, a.transport, a2.transport)
}

func TestBoltTransportScopedByHub(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mercure.db")

	a := &Bolt{Path: path}
	require.NoError(t, a.Provision(hubContext(t, "a")))
	t.Cleanup(func() { assert.NoError(t, a.Cleanup()) })

	a2 := &Bolt{Path: path}
	require.NoError(t, a2.Provision(hubContext(t, "a")))
	t.Cleanup(func() { assert.NoError(t, a2.Cleanup()) })
	assert.Same(t, a.transport, a2.transport)

	b := &Bolt{Path: path}
	require.ErrorContains(t, b.Provision(hubContext(t, "b")), "give each hub its own path")
}
