package hub

import (
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	bolt "go.etcd.io/bbolt"
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
			if len(hub.subscribers) == 0 {
				continue
			}

			hub.updates <- newSerializedUpdate(&Update{
				Topics: []string{"http://example.com/not-subscribed"},
				Event:  Event{Data: "Hello World", ID: "a"},
			})
			hub.updates <- newSerializedUpdate(&Update{
				Topics: []string{"http://example.com/books/1"},
				Event:  Event{Data: "Hello World", ID: "b"},
			})
			hub.updates <- newSerializedUpdate(&Update{
				Topics: []string{"http://example.com/reviews/22"},
				Event:  Event{Data: "Great", ID: "c"},
			})

			hub.Stop()
			return
		}
	}()

	req := httptest.NewRequest("GET", "http://example.com/subscribe?topic=http://example.com/books/1&topic=http://example.com/reviews/{id}", nil)
	w := newCloseNotifyingRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "id: b\ndata: Hello World\n\nid: c\ndata: Great\n\n", w.Body.String())
}

func TestUnsubscribe(t *testing.T) {
	hub := createAnonymousDummy()
	hub.Start()
	assert.Equal(t, 0, len(hub.subscribers))
	wr := newCloseNotifyingRecorder()

	var wg sync.WaitGroup
	wg.Add(1)
	go func(w *sync.WaitGroup) {
		defer w.Done()
		req := httptest.NewRequest("GET", "http://example.com/subscribe?topic=http://example.com/books/1", nil)
		hub.SubscribeHandler(wr, req)
		assert.Equal(t, 0, len(hub.subscribers))
	}(&wg)

	for {
		if len(hub.subscribers) != 0 {
			break
		}
	}

	wr.close()
	wg.Wait()
}

func TestSubscribeTarget(t *testing.T) {
	hub := createDummy()
	hub.Start()

	go func() {
		for {
			if len(hub.subscribers) == 0 {
				continue
			}

			hub.updates <- newSerializedUpdate(&Update{
				Targets: map[string]struct{}{"baz": {}},
				Topics:  []string{"http://example.com/reviews/21"},
				Event:   Event{Data: "Foo", ID: "a"},
			})
			hub.updates <- newSerializedUpdate(&Update{
				Targets: map[string]struct{}{},
				Topics:  []string{"http://example.com/reviews/22"},
				Event:   Event{Data: "Hello World", ID: "b", Type: "test"},
			})
			hub.updates <- newSerializedUpdate(&Update{
				Targets: map[string]struct{}{"hello": {}, "bar": {}},
				Topics:  []string{"http://example.com/reviews/23"},
				Event:   Event{Data: "Great", ID: "c", Retry: 1},
			})

			hub.Stop()
			return
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

func TestSendMissedEvents(t *testing.T) {
	db, _ := bolt.Open("test.db", 0600, nil)
	defer db.Close()
	defer os.Remove("test.db")

	history := &BoltHistory{db}
	history.Add(&Update{
		Topics: []string{"http://example.com/foos/a"},
		Event: Event{
			ID:   "a",
			Data: "d1",
		},
	})
	history.Add(&Update{
		Topics: []string{"http://example.com/foos/b"},
		Event: Event{
			ID:   "b",
			Data: "d2",
		},
	})

	hub := createAnonymousDummyWithHistory(history)
	hub.Start()

	var wg sync.WaitGroup
	wg.Add(2)

	wr1 := newCloseNotifyingRecorder()
	go func(w *sync.WaitGroup) {
		defer w.Done()
		req := httptest.NewRequest("GET", "http://example.com/subscribe?topic=http://example.com/foos/{id}&Last-Event-ID=a", nil)
		hub.SubscribeHandler(wr1, req)
		assert.Equal(t, "id: b\ndata: d2\n\n", wr1.Body.String())
	}(&wg)

	wr2 := newCloseNotifyingRecorder()
	go func(w *sync.WaitGroup) {
		defer w.Done()
		req := httptest.NewRequest("GET", "http://example.com/subscribe?topic=http://example.com/foos/{id}", nil)
		req.Header.Add("Last-Event-ID", "a")
		hub.SubscribeHandler(wr2, req)
		assert.Equal(t, "id: b\ndata: d2\n\n", wr2.Body.String())
	}(&wg)

	for {
		if len(hub.subscribers) == 2 {
			break
		}
	}

	wr1.close()
	wr2.close()
	wg.Wait()
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
