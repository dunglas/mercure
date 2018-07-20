package hub

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubscribeInvalidJWT(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("GET", "http://example.com/subscribe", nil)
	w := httptest.NewRecorder()
	http.SetCookie(w, &http.Cookie{Name: "mercureAuthorization", Value: "invalid"})
	hub.PublishHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

func TestSubscribeUnauthorizedJWT(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("GET", "http://example.com/subscribe", nil)
	w := httptest.NewRecorder()
	http.SetCookie(w, &http.Cookie{Name: "mercureAuthorization", Value: createDummyUnauthorizedJWT(hub)})
	hub.PublishHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

func TestSubscribeNoIRI(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("GET", "http://example.com/subscribe", nil)
	w := httptest.NewRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "Missing \"iri[]\" parameters.\n", w.Body.String())
}

func TestSubscribe(t *testing.T) {
	hub := createDummy()
	hub.Start()

	go func() {
		for {
			if len(hub.subscribers) > 0 {
				hub.resources <- NewResource("http://example.com/not-subscribed", "Hello World", map[string]bool{})
				hub.resources <- NewResource("http://example.com/books/1", "Hello World", map[string]bool{})
				hub.Stop()

				return
			}
		}
	}()

	req := httptest.NewRequest("GET", "http://example.com/subscribe?iri[]=http://example.com/books/1", nil)
	w := newCloseNotifyingRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "event: mercure\nid: http://example.com/books/1\ndata: Hello World\n\n", w.Body.String())
}

// From https://github.com/go-martini/martini/blob/master/response_writer_test.go
type closeNotifyingRecorder struct {
	*httptest.ResponseRecorder
	closed chan bool
}

func newCloseNotifyingRecorder() *closeNotifyingRecorder {
	return &closeNotifyingRecorder{
		httptest.NewRecorder(),
		make(chan bool, 1),
	}
}

func (c *closeNotifyingRecorder) close() {
	c.closed <- true
}

func (c *closeNotifyingRecorder) CloseNotify() <-chan bool {
	return c.closed
}
