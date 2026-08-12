package mercure

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDebuggerConfigHandler(t *testing.T) {
	t.Parallel()

	h, err := NewHub(t.Context(), WithDebugger(), WithPlayground(), WithAnonymous(), WithSubscriptions())
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "https://example.com/.well-known/mercure/debug/config.json", nil)
	w := httptest.NewRecorder()
	h.DebuggerConfigHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() { _ = resp.Body.Close() })

	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var cfg debuggerConfig
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&cfg))
	assert.True(t, cfg.Playground)
	assert.True(t, cfg.Anonymous)
	assert.True(t, cfg.Subscriptions)
	assert.Equal(t, defaultCookieName, cfg.CookieName)
}

func TestPlaygroundTokenHandlerUsesDerivedResourceIdentifier(t *testing.T) {
	t.Parallel()

	const resourceIdentifier = "https://hub.example.com/.well-known/mercure"

	h, err := NewHub(t.Context(),
		WithPlayground(),
		WithResourceIdentifier(resourceIdentifier),
		WithPlaygroundTokenFunc(func(rid string) (string, error) { return "token-for:" + rid, nil }),
	)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "https://example.com/.well-known/mercure/debug/playground-token", nil)
	w := httptest.NewRecorder()
	h.PlaygroundTokenHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() { _ = resp.Body.Close() })

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "token-for:"+resourceIdentifier, string(body))
}

// TestDebugRoutes checks the router wiring: config.json rides along with the
// debugger, and playground-token exists only when the playground is on and a
// minter is configured.
func TestDebugRoutes(t *testing.T) {
	t.Parallel()

	tokenFunc := WithPlaygroundTokenFunc(func(string) (string, error) { return "t", nil })

	for _, tc := range []struct {
		name       string
		opts       []Option
		path       string
		wantStatus int
	}{
		{"config.json with debugger", []Option{WithDebugger()}, "debug/config.json", http.StatusOK},
		{"config.json without debugger", nil, "debug/config.json", http.StatusNotFound},
		{"playground-token with minter", []Option{WithPlayground(), tokenFunc}, "debug/playground-token", http.StatusOK},
		{"playground-token without minter", []Option{WithPlayground()}, "debug/playground-token", http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h, err := NewHub(t.Context(), tc.opts...)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodGet, "https://example.com/.well-known/mercure/"+tc.path, nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)

			assert.Equal(t, tc.wantStatus, w.Result().StatusCode)
		})
	}
}
