package mercure

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/synctest"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// hubShutdownTestHub builds a hub with a caller-controlled context so tests
// can cancel the hub independently of the subscriber's request context.
func hubShutdownTestHub(ctx context.Context, tb testing.TB, writeTimeout time.Duration) *Hub {
	tb.Helper()

	tss, err := NewTopicSelectorStore(0)
	require.NoError(tb, err)

	h, err := NewHub(ctx,
		WithAnonymous(),
		WithPublisherJWT([]byte("publisher"), jwt.SigningMethodHS256.Name),
		WithSubscriberJWT([]byte("subscriber"), jwt.SigningMethodHS256.Name),
		WithTopicSelectorStore(tss),
		WithWriteTimeout(writeTimeout),
	)
	require.NoError(tb, err)
	setDeprecatedOptions(tb, h)

	return h
}

// TestShutdownKeepsSubscribersWhenWriteTimeoutEnabled verifies the graceful
// drain contract: when the hub context is cancelled (Caddy stopping, pod
// SIGTERM, ...) and writeTimeout is set, subscribers stay connected until
// their per-connection disconnection timer fires or the client disconnects.
func TestShutdownKeepsSubscribersWhenWriteTimeoutEnabled(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		hubCtx, cancelHub := context.WithCancel(t.Context())
		hub := hubShutdownTestHub(hubCtx, t, 5*time.Minute)
		transport, _ := hub.transport.(*LocalTransport)

		go func() {
			req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match=https://example.com/books/1", nil).WithContext(t.Context())
			hub.SubscribeHandler(newSubscribeRecorder(), req)
		}()

		waitSubscribers(t, transport, 1)

		// Simulate hub shutdown.
		cancelHub()
		synctest.Wait()

		transport.RLock()
		n := transport.subscribers.Len()
		transport.RUnlock()
		assert.Equal(t, 1, n, "subscriber must stay connected when writeTimeout is set; disconnect timer is the drain mechanism")
	})
}

// TestShutdownClosesSubscribersWhenWriteTimeoutDisabled covers the escape
// hatch: with writeTimeout == 0 there is no per-connection disconnect timer,
// so the hub context cancel must still terminate subscribers — otherwise
// http.Server.Shutdown would hang forever on active handlers.
func TestShutdownClosesSubscribersWhenWriteTimeoutDisabled(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		hubCtx, cancelHub := context.WithCancel(t.Context())
		hub := hubShutdownTestHub(hubCtx, t, 0)
		transport, _ := hub.transport.(*LocalTransport)

		go func() {
			req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match=https://example.com/books/1", nil).WithContext(t.Context())
			hub.SubscribeHandler(newSubscribeRecorder(), req)
		}()

		waitSubscribers(t, transport, 1)

		cancelHub()
		synctest.Wait()

		transport.RLock()
		n := transport.subscribers.Len()
		transport.RUnlock()
		assert.Equal(t, 0, n, "subscriber must exit on hub shutdown when writeTimeout is 0")
	})
}
