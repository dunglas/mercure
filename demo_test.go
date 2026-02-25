package mercure

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEmptyBodyAndJWT(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "https://example.com/demo/foo.jsonld", nil)
	w := httptest.NewRecorder()

	h, _ := NewHub(t.Context())
	h.Demo(w, req)

	resp := w.Result()
	assert.Equal(t, "application/ld+json", resp.Header.Get("Content-Type"))
	assert.Equal(t, []string{hubLink, `<https://example.com/demo/foo.jsonld>; rel="self"`}, resp.Header["Link"])

	cookie := resp.Cookies()[0]
	assert.Equal(t, "mercureAuthorization", cookie.Name)
	assert.Empty(t, cookie.Value)
	assert.True(t, cookie.Expires.Before(time.Now()))
	assert.True(t, cookie.HttpOnly)
	assert.Equal(t, http.SameSiteStrictMode, cookie.SameSite)

	t.Cleanup(func() {
		_ = resp.Body.Close()
	})

	body, _ := io.ReadAll(resp.Body)
	assert.Empty(t, string(body))
}

func TestBodyAndJWT(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "https://example.com/demo/foo/bar.xml?body=<hello/>&jwt=token", nil)
	w := httptest.NewRecorder()

	h, _ := NewHub(t.Context())
	h.Demo(w, req)

	resp := w.Result()
	assert.Contains(t, resp.Header.Get("Content-Type"), "xml") // Before Go 1.17, the charset wasn't set
	assert.Equal(t, []string{hubLink, `<https://example.com/demo/foo/bar.xml?body=<hello/>&jwt=token>; rel="self"`}, resp.Header["Link"])

	cookie := resp.Cookies()[0]
	assert.Equal(t, "mercureAuthorization", cookie.Name)
	assert.Equal(t, "token", cookie.Value)
	assert.Empty(t, cookie.Expires)
	assert.True(t, cookie.HttpOnly)
	assert.Equal(t, http.SameSiteStrictMode, cookie.SameSite)

	t.Cleanup(func() {
		_ = resp.Body.Close()
	})

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "<hello/>", string(body))
}

func TestDemoCookieSecureWithTLS(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "https://example.com/demo/foo.jsonld?jwt=token", nil)
	req.TLS = &tls.ConnectionState{}
	w := httptest.NewRecorder()

	h, _ := NewHub(t.Context())
	h.Demo(w, req)

	cookie := w.Result().Cookies()[0]
	assert.True(t, cookie.Secure)
	assert.True(t, cookie.HttpOnly)
}

func TestDemoCookieSecureWithXForwardedProto(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "http://example.com/demo/foo.jsonld?jwt=token", nil)
	req.Header.Set("X-Forwarded-Proto", "https")

	w := httptest.NewRecorder()

	h, _ := NewHub(t.Context())
	h.Demo(w, req)

	cookie := w.Result().Cookies()[0]
	assert.True(t, cookie.Secure)
	assert.True(t, cookie.HttpOnly)
}

func TestDemoCookieNotSecureOnPlainHTTP(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "http://example.com/demo/foo.jsonld?jwt=token", nil)
	w := httptest.NewRecorder()

	h, _ := NewHub(t.Context())
	h.Demo(w, req)

	cookie := w.Result().Cookies()[0]
	assert.False(t, cookie.Secure)
	assert.True(t, cookie.HttpOnly)
}
