package hub

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

type responseWriterMock struct {
}

func (m *responseWriterMock) Header() http.Header {
	return http.Header{}
}

func (m *responseWriterMock) Write([]byte) (int, error) {
	return 0, nil
}

func (m *responseWriterMock) WriteHeader(statusCode int) {
}

func TestSubscribeNotAFlusher(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("GET", "http://example.com/subscribe", nil)

	assert.PanicsWithValue(t, "The Reponse Writter must be an instance of Flusher.", func() {
		hub.SubscribeHandler(&responseWriterMock{}, req)
	})
}

func TestSubscribeNoCookie(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("GET", "http://example.com/subscribe", nil)
	w := httptest.NewRecorder()

	hub.SubscribeHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

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
	http.SetCookie(w, &http.Cookie{Name: "mercureAuthorization", Value: createDummyUnauthorizedJWT()})
	req.Header = http.Header{"Cookie": w.HeaderMap["Set-Cookie"]}

	hub.SubscribeHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

func TestSubscribeInvalidAlgJWT(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("GET", "http://example.com/subscribe", nil)
	w := httptest.NewRecorder()
	http.SetCookie(w, &http.Cookie{Name: "mercureAuthorization", Value: createDummyNoneSignedJWT()})
	req.Header = http.Header{"Cookie": w.HeaderMap["Set-Cookie"]}

	hub.SubscribeHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

func TestSubscribeNoTopic(t *testing.T) {
	hub := createAnonymousDummy()

	req := httptest.NewRequest("GET", "http://example.com/subscribe", nil)
	w := httptest.NewRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "Missing \"topic\" parameter.\n", w.Body.String())
}

func TestSubscribeInvalidIRI(t *testing.T) {
	hub := createAnonymousDummy()

	req := httptest.NewRequest("GET", "http://example.com/subscribe?topic=fau{lty", nil)
	w := httptest.NewRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "\"fau{lty\" is not a valid URI template (RFC6570).\n", w.Body.String())
}

func TestSubscribe(t *testing.T) {
	hub := createAnonymousDummy()
	hub.Start()

	go func() {
		for {
			if len(hub.subscribers) > 0 {
				hub.updates <- newSerializedUpdate(NewUpdate([]string{"http://example.com/not-subscribed"}, map[string]struct{}{}, "Hello World", "a", "", 0))
				hub.updates <- newSerializedUpdate(NewUpdate([]string{"http://example.com/books/1"}, map[string]struct{}{}, "Hello World", "b", "", 0))
				hub.updates <- newSerializedUpdate(NewUpdate([]string{"http://example.com/reviews/22"}, map[string]struct{}{}, "Great", "c", "", 0))
				hub.Stop()

				return
			}
		}
	}()

	req := httptest.NewRequest("GET", "http://example.com/subscribe?topic=http://example.com/books/1&topic=http://example.com/reviews/{id}", nil)
	w := newCloseNotifyingRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "id: b\ndata: Hello World\n\nid: c\ndata: Great\n\n", w.Body.String())
}

func TestSubscribeTarget(t *testing.T) {
	hub := createDummy()
	hub.Start()

	go func() {
		for {
			if len(hub.subscribers) > 0 {
				hub.updates <- newSerializedUpdate(NewUpdate([]string{"http://example.com/reviews/21"}, map[string]struct{}{"baz": struct{}{}}, "Foo", "a", "", 0))
				hub.updates <- newSerializedUpdate(NewUpdate([]string{"http://example.com/reviews/22"}, map[string]struct{}{}, "Hello World", "b", "test", 0))
				hub.updates <- newSerializedUpdate(NewUpdate([]string{"http://example.com/reviews/23"}, map[string]struct{}{"hello": struct{}{}, "bar": struct{}{}}, "Great", "c", "", 1))
				hub.Stop()

				return
			}
		}
	}()

	req := httptest.NewRequest("GET", "http://example.com/subscribe?topic=http://example.com/reviews/{id}", nil)
	w := newCloseNotifyingRecorder()
	http.SetCookie(w, &http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWTWithTargets(hub, []string{"foo", "bar"})})
	req.Header = http.Header{"Cookie": w.HeaderMap["Set-Cookie"]}

	http.SetCookie(w, &http.Cookie{Name: "mercureAuthorization", Value: createDummyUnauthorizedJWT()})

	hub.SubscribeHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "event: test\nid: b\ndata: Hello World\n\nretry: 1\nid: c\ndata: Great\n\n", w.Body.String())
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
