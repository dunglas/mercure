package mercure

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSpansArbitraryOrigins(t *testing.T) {
	t.Parallel()

	for origin, spans := range map[string]bool{
		"https://example.com":   false,
		"https://*.example.com": false,
		"null":                  false,
		"*":                     true,
		"https://*":             true,
		"https://*example.com":  true,
		"http*://example.com":   true,
		"https://*.*.com":       true,
	} {
		assert.Equal(t, spans, spansArbitraryOrigins(origin), origin)
	}
}

func TestCORSWildcardCredentials(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		pattern     string
		origin      string
		credentials bool
	}{
		{"https://*.com", "https://attacker.com", false},
		{"https://*.co.uk", "https://attacker.co.uk", false},
		{"https://*.github.io", "https://attacker.github.io", false},
		{"https://*.COM", "https://attacker.com", false},
		{"https://*.com:8443", "https://attacker.com:8443", false},
		{"https://*.example.com", "https://app.example.com", true},
		{"https://*.example.co.uk:8443", "https://app.example.co.uk:8443", true},
		{"https://*.project.github.io", "https://app.project.github.io", true},
	} {
		t.Run(tc.pattern, func(t *testing.T) {
			t.Parallel()

			h := &Hub{opt: &opt{corsOrigins: []string{tc.pattern}}}
			handler := h.corsHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Origin", tc.origin)

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.Equal(t, tc.origin, w.Header().Get("Access-Control-Allow-Origin"))
			assert.Equal(t, tc.credentials, w.Header().Get("Access-Control-Allow-Credentials") == "true")
		})
	}
}
