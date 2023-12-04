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
	req := httptest.NewRequest(http.MethodGet, "http://example.com/demo/foo.jsonld", nil)
	w := httptest.NewRecorder()

	h, _ := NewHub()
	h.Demo(w, req)

	resp := w.Result()
	assert.Equal(t, "application/ld+json", resp.Header.Get("Content-Type"))
	assert.Equal(t, []string{"<" + defaultHubURL + linkSuffix, "<http://example.com/demo/foo.jsonld>; rel=\"self\""}, resp.Header["Link"])

	cookie := resp.Cookies()[0]
	assert.Equal(t, "mercureAuthorization", cookie.Name)
	assert.Empty(t, cookie.Value)
	assert.True(t, cookie.Expires.Before(time.Now()))

	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "", string(body))
}

func TestBodyAndJWT(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.com/demo/foo/bar.xml?body=<hello/>&jwt=token", nil)
	w := httptest.NewRecorder()

	h, _ := NewHub()
	h.Demo(w, req)

	resp := w.Result()
	assert.Contains(t, resp.Header.Get("Content-Type"), "xml") // Before Go 1.17, the charset wasn't set
	assert.Equal(t, []string{"<" + defaultHubURL + linkSuffix, "<http://example.com/demo/foo/bar.xml?body=<hello/>&jwt=token>; rel=\"self\""}, resp.Header["Link"])

	cookie := resp.Cookies()[0]
	assert.Equal(t, "mercureAuthorization", cookie.Name)
	assert.Equal(t, "token", cookie.Value)
	assert.Empty(t, cookie.Expires)

	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "<hello/>", string(body))
}
