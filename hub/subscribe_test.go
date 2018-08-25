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
	req.Header = http.Header{"Cookie": w.HeaderMap["Set-Cookie"]}

	hub.SubscribeHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

func TestSubscribeUnauthorizedJWT(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("GET", "http://example.com/subscribe", nil)
	w := httptest.NewRecorder()
	http.SetCookie(w, &http.Cookie{Name: "mercureAuthorization", Value: createDummyUnauthorizedJWT(hub)})
	req.Header = http.Header{"Cookie": w.HeaderMap["Set-Cookie"]}

	hub.SubscribeHandler(w, req)

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
				hub.resources <- NewResource("a", "http://example.com/not-subscribed", "Hello World", map[string]struct{}{})
				hub.resources <- NewResource("b", "http://example.com/books/1", "Hello World", map[string]struct{}{})
				hub.resources <- NewResource("c", "http://example.com/reviews/22", "Great", map[string]struct{}{})
				hub.Stop()

				return
			}
		}
	}()

	req := httptest.NewRequest("GET", "http://example.com/subscribe?iri[]=http://example.com/books/1&iri[]=http://example.com/reviews/{id}", nil)
	w := newCloseNotifyingRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "event: mercure\nid: b\ndata: Hello World\n\nevent: mercure\nid: c\ndata: Great\n\n", w.Body.String())
}

func TestSubscribeTarget(t *testing.T) {
	hub := createDummy()
	hub.Start()

	go func() {
		for {
			if len(hub.subscribers) > 0 {
				hub.resources <- NewResource("a", "http://example.com/reviews/21", "Foo", map[string]struct{}{"baz": struct{}{}})
				hub.resources <- NewResource("b", "http://example.com/reviews/22", "Hello World", map[string]struct{}{})
				hub.resources <- NewResource("c", "http://example.com/reviews/23", "Great", map[string]struct{}{"hello": struct{}{}, "bar": struct{}{}})
				hub.Stop()

				return
			}
		}
	}()

	req := httptest.NewRequest("GET", "http://example.com/subscribe?iri[]=http://example.com/reviews/{id}", nil)
	w := newCloseNotifyingRecorder()
	http.SetCookie(w, &http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWTWithTargets(hub, []string{"foo", "bar"})})
	req.Header = http.Header{"Cookie": w.HeaderMap["Set-Cookie"]}

	http.SetCookie(w, &http.Cookie{Name: "mercureAuthorization", Value: createDummyUnauthorizedJWT(hub)})

	hub.SubscribeHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "event: mercure\nid: b\ndata: Hello World\n\nevent: mercure\nid: c\ndata: Great\n\n", w.Body.String())
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
