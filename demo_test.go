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

	req := httptest.NewRequest(http.MethodGet, "https://example.com/demo/foo.jsonld", nil)
	w := httptest.NewRecorder()

	h, _ := NewHub(t.Context())
	h.Demo(w, req)

	resp := w.Result()
	assert.Equal(t, "application/ld+json", resp.Header.Get("Content-Type"))
	assert.Equal(t, []string{"<" + defaultHubURL + linkSuffix, "<https://example.com/demo/foo.jsonld>; rel=\"self\""}, resp.Header["Link"])

	cookie := resp.Cookies()[0]
	assert.Equal(t, "mercureAuthorization", cookie.Name)
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

	req := httptest.NewRequest(http.MethodGet, "https://example.com/demo/foo/bar.xml?body=<hello/>&jwt=token", nil)
	w := httptest.NewRecorder()

	h, _ := NewHub(t.Context())
	h.Demo(w, req)

	resp := w.Result()
	assert.Contains(t, resp.Header.Get("Content-Type"), "xml") // Before Go 1.17, the charset wasn't set
	assert.Equal(t, []string{"<" + defaultHubURL + linkSuffix, "<https://example.com/demo/foo/bar.xml?body=<hello/>&jwt=token>; rel=\"self\""}, resp.Header["Link"])

	cookie := resp.Cookies()[0]
	assert.Equal(t, "mercureAuthorization", cookie.Name)
	assert.Equal(t, "token", cookie.Value)
	assert.Empty(t, cookie.Expires)

	t.Cleanup(func() {
		_ = resp.Body.Close()
	})

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "<hello/>", string(body))
}
