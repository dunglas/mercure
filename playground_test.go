package mercure

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
	assert.Contains(t, resp.Header.Get("Content-Type"), "xml") // Before Go 1.17, the charset wasn't set
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
