package hub

import (
	"io/ioutil"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEmptyBodyAndJWT(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/demo/foo.jsonld", nil)
	w := httptest.NewRecorder()
	Demo(w, req)

	resp := w.Result()
	assert.Equal(t, "application/ld+json", resp.Header.Get("Content-Type"))
	assert.Equal(t, []string{"<" + defaultHubURL + ">; rel=\"mercure\"", "<http://example.com/demo/foo.jsonld>; rel=\"self\""}, resp.Header["Link"])

	cookie := resp.Cookies()[0]
	assert.Equal(t, "mercureAuthorization", cookie.Name)
	assert.Empty(t, cookie.Value)
	assert.True(t, cookie.Expires.Before(time.Now()))

	body, _ := ioutil.ReadAll(resp.Body)
	assert.Equal(t, "", string(body))
}

func TestBodyAndJWT(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/demo/foo/bar.xml?body=<hello/>&jwt=token", nil)
	w := httptest.NewRecorder()
	Demo(w, req)

	resp := w.Result()
	assert.Equal(t, "application/xml", resp.Header.Get("Content-Type"))
	assert.Equal(t, []string{"<" + defaultHubURL + ">; rel=\"mercure\"", "<http://example.com/demo/foo/bar.xml?body=<hello/>&jwt=token>; rel=\"self\""}, resp.Header["Link"])

	cookie := resp.Cookies()[0]
	assert.Equal(t, "mercureAuthorization", cookie.Name)
	assert.Equal(t, "token", cookie.Value)
	assert.Empty(t, cookie.Expires)

	body, _ := ioutil.ReadAll(resp.Body)
	assert.Equal(t, "<hello/>", string(body))
}
