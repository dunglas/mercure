package hub

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNoAuthorizationHeader(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("GET", "http://example.com/publish", nil)
	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

func TestPublishInvalidJWT(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("GET", "http://example.com/publish", nil)
	req.Header.Add("Authorization", "Bearer invalid")
	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}
func TestPublishUnauthorizedJWT(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("GET", "http://example.com/publish", nil)
	req.Header.Add("Authorization", "Bearer "+createDummyUnauthorizedJWT(hub))
	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

func TestPublishNoIRI(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("GET", "http://example.com/publish", nil)
	req.Header.Add("Authorization", "Bearer "+createDummyAuthorizedJWT(hub, true))
	req.Form = url.Values{}
	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "Missing \"iri\" parameter\n", w.Body.String())
}

func TestPublishNoData(t *testing.T) {
	hub := createDummy()

	form := url.Values{}
	form.Add("iri", "http://example.com/books/1")

	req := httptest.NewRequest("GET", "http://example.com/publish", nil)
	req.Header.Add("Authorization", "Bearer "+createDummyAuthorizedJWT(hub, true))
	req.Form = form
	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "Missing \"data\" parameter\n", w.Body.String())
}

func TestPublishOk(t *testing.T) {
	hub := createDummy()

	var wg sync.WaitGroup
	wg.Add(1)
	go func(w *sync.WaitGroup) {
		defer w.Done()
		for {
			select {
			case content := <-hub.resources:
				assert.Equal(t, "http://example.com/books/1", content.IRI)
				assert.Equal(t, "data: Hello!\n\n", content.Data)
				assert.True(t, content.Targets["foo"])
				assert.True(t, content.Targets["bar"])
				return
			}
		}
	}((&wg))

	form := url.Values{}
	form.Add("iri", "http://example.com/books/1")
	form.Add("data", "Hello!")
	form.Add("target[]", "foo")
	form.Add("target[]", "bar")

	req := httptest.NewRequest("GET", "http://example.com/publish", nil)
	req.Header.Add("Authorization", "Bearer "+createDummyAuthorizedJWT(hub, true))
	req.Form = form
	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	wg.Wait()
}
