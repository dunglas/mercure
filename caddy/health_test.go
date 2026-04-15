package caddy

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/caddyserver/caddy/v2/caddytest"
	"github.com/dunglas/mercure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthEndpoints(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`{
	skip_install_trust
	admin localhost:2999
	http_port     9080
	https_port    9443
}

localhost:9080 {
	route {
		mercure {
			anonymous
			publisher_jwt !ChangeMe!
			transport local
		}

		respond 404
	}
}`, "caddyfile")

	tests := []struct {
		name string
		path string
	}{
		{"aggregate ready", "/mercure/health/ready"},
		{"aggregate live", "/mercure/health/live"},
		{"per-hub ready", "/mercure/health/default/ready"},
		{"per-hub live", "/mercure/health/default/live"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, "http://localhost:2999"+tt.path, nil)
			require.NoError(t, err)

			resp := tester.AssertResponseCode(req, http.StatusOK)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, resp.Body.Close())
			require.NoError(t, err)

			assert.Contains(t, string(body), `"status":"ok"`)
		})
	}
}

func TestHealthEndpointNotFound(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`{
	skip_install_trust
	admin localhost:2999
	http_port     9080
	https_port    9443
}

localhost:9080 {
	route {
		mercure {
			anonymous
			publisher_jwt !ChangeMe!
			transport local
		}

		respond 404
	}
}`, "caddyfile")

	req, err := http.NewRequest(http.MethodGet, "http://localhost:2999/mercure/health/invalid", nil)
	require.NoError(t, err)

	resp := tester.AssertResponseCode(req, http.StatusNotFound)
	require.NoError(t, resp.Body.Close())
}

func TestHealthEndpointMethodNotAllowed(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`{
	skip_install_trust
	admin localhost:2999
	http_port     9080
	https_port    9443
}

localhost:9080 {
	route {
		mercure {
			anonymous
			publisher_jwt !ChangeMe!
			transport local
		}

		respond 404
	}
}`, "caddyfile")

	req, err := http.NewRequest(http.MethodPost, "http://localhost:2999/mercure/health/ready", nil)
	require.NoError(t, err)

	resp := tester.AssertResponseCode(req, http.StatusMethodNotAllowed)
	require.NoError(t, resp.Body.Close())
}

// failingTransport is mock transport that implements TransportHealthChecker
// and always reports unhealthy.
type failingTransport struct {
	mercure.Transport
}

var (
	errMockNotReady = errors.New("connection refused")
	errMockNotLive  = errors.New("unhealthy for 30s")
)

func (failingTransport) Ready(_ context.Context) error {
	return errMockNotReady
}

func (failingTransport) Live(_ context.Context) error {
	return errMockNotLive
}

func TestHealthEndpointUnhealthy(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`{
	skip_install_trust
	admin localhost:2999
	http_port     9080
	https_port    9443
}

localhost:9080 {
	route {
		mercure {
			anonymous
			publisher_jwt !ChangeMe!
			transport local
		}

		respond 404
	}
}`, "caddyfile")

	// Inject a failing transport into the hubs map to simulate transport failure
	hubsMu.Lock()

	originalInfos := make([]*hubInfo, 0, len(hubs))
	for _, info := range hubs {
		originalInfos = append(originalInfos, info)
	}

	for k, info := range hubs {
		hubs[k] = &hubInfo{
			hub:       info.hub,
			transport: failingTransport{},
			name:      info.name,
		}
	}
	hubsMu.Unlock()

	t.Cleanup(func() {
		hubsMu.Lock()
		for k, info := range hubs {
			for _, orig := range originalInfos {
				if info.name == orig.name {
					hubs[k] = orig

					break
				}
			}
		}
		hubsMu.Unlock()
	})

	// Ready should return 503 with a generic error message
	// (detailed errors are logged server-side, not exposed in the response).
	req, err := http.NewRequest(http.MethodGet, "http://localhost:2999/mercure/health/ready", nil)
	require.NoError(t, err)

	resp := tester.AssertResponseCode(req, http.StatusServiceUnavailable)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, resp.Body.Close())
	require.NoError(t, err)
	assert.Contains(t, string(body), `"status":"error"`)
	assert.Contains(t, string(body), "transport health check failed")
	assert.NotContains(t, string(body), "connection refused") // internal detail must not leak

	// Live should return 503 with a generic error message
	req, err = http.NewRequest(http.MethodGet, "http://localhost:2999/mercure/health/live", nil)
	require.NoError(t, err)

	resp = tester.AssertResponseCode(req, http.StatusServiceUnavailable)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, resp.Body.Close())
	require.NoError(t, err)
	assert.Contains(t, string(body), `"status":"error"`)
	assert.Contains(t, string(body), "transport health check failed")
	assert.NotContains(t, string(body), "unhealthy for 30s") // internal detail must not leak
}

func TestHealthEndpointUnknownHub(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`{
	skip_install_trust
	admin localhost:2999
	http_port     9080
	https_port    9443
}

localhost:9080 {
	route {
		mercure {
			anonymous
			publisher_jwt !ChangeMe!
			transport local
		}

		respond 404
	}
}`, "caddyfile")

	// Querying a hub name that doesn't exist should return 404, not 200.
	req, err := http.NewRequest(http.MethodGet, "http://localhost:2999/mercure/health/nonexistent/ready", nil)
	require.NoError(t, err)

	resp := tester.AssertResponseCode(req, http.StatusNotFound)
	require.NoError(t, resp.Body.Close())
}
