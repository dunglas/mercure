package mercure

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmptyBodyAndJWT(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "https://example.com/playground/foo.jsonld", nil)
	w := httptest.NewRecorder()

	h, _ := NewHub(t.Context())
	h.Playground(w, req)

	resp := w.Result()
	assert.Equal(t, "application/ld+json", resp.Header.Get("Content-Type"))
	assert.Equal(t, []string{hubLink, `<https://example.com/playground/foo.jsonld>; rel="self"`}, resp.Header["Link"])

	cookie := resp.Cookies()[0]
	assert.Equal(t, defaultCookieName, cookie.Name)
	assert.Empty(t, cookie.Value)
	assert.True(t, cookie.Expires.Before(time.Now()))

	t.Cleanup(func() {
		_ = resp.Body.Close()
	})

	body, _ := io.ReadAll(resp.Body)
	assert.Empty(t, string(body))
}

func TestBodyAndJWT(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "https://example.com/playground/foo/bar.xml?body=<hello/>&jwt=token", nil)
	w := httptest.NewRecorder()

	h, _ := NewHub(t.Context())
	h.Playground(w, req)

	resp := w.Result()
	assert.Equal(t, "text/plain; charset=utf-8", resp.Header.Get("Content-Type"))
	assert.Equal(t, []string{hubLink, `<https://example.com/playground/foo/bar.xml?body=%3Chello/%3E&jwt=token>; rel="self"`}, resp.Header["Link"])

	cookie := resp.Cookies()[0]
	assert.Equal(t, defaultCookieName, cookie.Name)
	assert.Equal(t, "token", cookie.Value)
	assert.Empty(t, cookie.Expires)

	t.Cleanup(func() {
		_ = resp.Body.Close()
	})

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "<hello/>", string(body))
}

func TestPlaygroundResponseFormats(t *testing.T) {
	t.Parallel()

	for _, debug := range []bool{false, true} {
		options := []Option{WithPlayground()}
		if debug {
			options = append(options, WithDebug())
		}

		h, err := NewHub(t.Context(), options...)
		require.NoError(t, err)

		for _, tc := range []struct {
			path        string
			contentType string
		}{
			{"topic.json", "application/json"},
			{"topic.jsonld", "application/ld+json"},
			{"topic.JSONLD", "application/ld+json"},
			{"topic.txt", "text/plain; charset=utf-8"},
			{"topic.html", "text/plain; charset=utf-8"},
			{"topic.js", "text/plain; charset=utf-8"},
			{"topic.svg", "text/plain; charset=utf-8"},
			{"topic.xml", "text/plain; charset=utf-8"},
			{"topic.unknown", "text/plain; charset=utf-8"},
			{"topic", "text/plain; charset=utf-8"},
		} {
			const body = `<script>document.cookie = "injected=1; Domain=example.com; Path=/"</script>`

			req := httptest.NewRequest(http.MethodGet, "https://example.com/playground/"+tc.path+"?body="+url.QueryEscape(body), nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code, tc.path)
			assert.Equal(t, tc.contentType, w.Header().Get("Content-Type"), tc.path)
			assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"), tc.path)
			assert.Equal(t, "sandbox; default-src 'none'", w.Header().Get("Content-Security-Policy"), tc.path)
			assert.Equal(t, body, w.Body.String(), tc.path)
		}

		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "https://example.com"+defaultDebugURL, nil))
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
		assert.Equal(t, "default-src 'self'", w.Header().Get("Content-Security-Policy"))
	}
}

func TestPlaygroundSelfLinkInjection(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, `https://example.com/playground/foo?a>;rel="mercure",<https://evil.example/hub`, nil)
	w := httptest.NewRecorder()

	h, _ := NewHub(t.Context())
	h.Playground(w, req)

	resp := w.Result()

	t.Cleanup(func() {
		_ = resp.Body.Close()
	})

	assert.Equal(t, []string{hubLink, `<https://example.com/playground/foo?a%3E;rel=%22mercure%22,%3Chttps://evil.example/hub>; rel="self"`}, resp.Header["Link"])
}

func TestEscapeLinkTarget(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "/p/a%20b?q=%3C%3E%22%5C%5E%60%7B%7C%7D%0D%0A%C3%A9", escapeLinkTarget("/p/a b?q=<>\"\\^`{|}\r\né"))
	assert.Equal(t, "https://u@h:1/p;x=1?a=%41&b=,;!$'()*+#f[]~", escapeLinkTarget("https://u@h:1/p;x=1?a=%41&b=,;!$'()*+#f[]~"))
}
