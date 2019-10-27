package hub

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
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

	req := httptest.NewRequest("GET", "http://example.com/hub", nil)

	assert.PanicsWithValue(t, "The Response Writer must be an instance of Flusher.", func() {
		hub.SubscribeHandler(&responseWriterMock{}, req)
	})
}

func TestSubscribeNoCookie(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("GET", "http://example.com/hub", nil)
	w := httptest.NewRecorder()

	hub.SubscribeHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

func TestSubscribeInvalidJWT(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("GET", "http://example.com/hub", nil)
	w := httptest.NewRecorder()
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: "invalid"})

	hub.SubscribeHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

func TestSubscribeUnauthorizedJWT(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("GET", "http://example.com/hub", nil)
	w := httptest.NewRecorder()
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyUnauthorizedJWT()})
	req.Header = http.Header{"Cookie": w.HeaderMap["Set-Cookie"]}

	hub.SubscribeHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

func TestSubscribeInvalidAlgJWT(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("GET", "http://example.com/hub", nil)
	w := httptest.NewRecorder()
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyNoneSignedJWT()})

	hub.SubscribeHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

func TestSubscribeNoTopic(t *testing.T) {
	hub := createAnonymousDummy()

	req := httptest.NewRequest("GET", "http://example.com/hub", nil)
	w := httptest.NewRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "Missing \"topic\" parameter.\n", w.Body.String())
}

func testSubscribe(numberOfSubscribers int, t *testing.T) {
	log.SetLevel(log.DebugLevel)
	hub := createAnonymousDummy()

	go func() {
		for {
			s, _ := hub.transport.(*LocalTransport)
			s.RLock()
			ready := len(s.pipes) == numberOfSubscribers
			s.RUnlock()

			if !ready {
				continue
			}

			hub.transport.Write(&Update{
				Topics: []string{"http://example.com/not-subscribed"},
				Event:  Event{Data: "Hello World", ID: "a"},
			})
			hub.transport.Write(&Update{
				Topics: []string{"http://example.com/books/1"},
				Event:  Event{Data: "Hello World", ID: "b"},
			})
			hub.transport.Write(&Update{
				Topics: []string{"http://example.com/reviews/22"},
				Event:  Event{Data: "Great", ID: "c"},
			})
			hub.transport.Write(&Update{
				Topics: []string{"http://example.com/hub?topic=faulty{iri"},
				Event:  Event{Data: "Faulty IRI", ID: "d"},
			})
			hub.transport.Write(&Update{
				Topics: []string{"string"},
				Event:  Event{Data: "string", ID: "e"},
			})

			time.Sleep(8 * time.Millisecond)
			hub.Stop()
			return
		}
	}()

	var wg sync.WaitGroup
	wg.Add(numberOfSubscribers)
	for i := 0; i < numberOfSubscribers; i++ {
		go func(w2 *sync.WaitGroup) {
			defer w2.Done()
			req := httptest.NewRequest("GET", "http://example.com/hub?topic=http://example.com/books/1&topic=string&topic=http://example.com/reviews/{id}&topic=http://example.com/hub?topic=faulty{iri", nil)
			w := httptest.NewRecorder()
			hub.SubscribeHandler(w, req)

			if t != nil {
				resp := w.Result()
				assert.Equal(t, http.StatusOK, resp.StatusCode)
				assert.Equal(t, ":\nid: b\ndata: Hello World\n\nid: c\ndata: Great\n\nid: d\ndata: Faulty IRI\n\nid: e\ndata: string\n\n", w.Body.String())
			}
		}(&wg)
	}

	wg.Wait()
}

func TestSubscribe(t *testing.T) {
	testSubscribe(3, t)
}

func TestUnsubscribe(t *testing.T) {
	hub := createAnonymousDummy()

	s, _ := hub.transport.(*LocalTransport)
	assert.Equal(t, 0, len(s.pipes))
	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)
	go func(w *sync.WaitGroup) {
		defer w.Done()
		req := httptest.NewRequest("GET", "http://example.com/hub?topic=http://example.com/books/1", nil).WithContext(ctx)
		hub.SubscribeHandler(httptest.NewRecorder(), req)
		assert.Equal(t, 1, len(s.pipes))
		for pipe := range s.pipes {
			assert.True(t, pipe.IsClosed())
		}
	}(&wg)

	for {
		s.RLock()
		notEmpty := len(s.pipes) != 0
		s.RUnlock()
		if notEmpty {
			break
		}
	}

	cancel()
	wg.Wait()
}

func TestSubscribeTarget(t *testing.T) {
	hub := createDummy()
	hub.options.Debug = true
	s, _ := hub.transport.(*LocalTransport)

	go func() {
		for {
			s.RLock()
			empty := len(s.pipes) == 0
			s.RUnlock()

			if empty {
				continue
			}

			hub.transport.Write(&Update{
				Targets: map[string]struct{}{"baz": {}},
				Topics:  []string{"http://example.com/reviews/21"},
				Event:   Event{Data: "Foo", ID: "a"},
			})
			hub.transport.Write(&Update{
				Targets: map[string]struct{}{},
				Topics:  []string{"http://example.com/reviews/22"},
				Event:   Event{Data: "Hello World", ID: "b", Type: "test"},
			})
			hub.transport.Write(&Update{
				Targets: map[string]struct{}{"hello": {}, "bar": {}},
				Topics:  []string{"http://example.com/reviews/23"},
				Event:   Event{Data: "Great", ID: "c", Retry: 1},
			})

			time.Sleep(8 * time.Millisecond)
			hub.Stop()
			return
		}
	}()

	req := httptest.NewRequest("GET", "http://example.com/hub?topic=http://example.com/reviews/{id}", nil)
	w := httptest.NewRecorder()
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(hub, false, []string{"foo", "bar"})})

	hub.SubscribeHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, ":\nevent: test\nid: b\ndata: Hello World\n\nretry: 1\nid: c\ndata: Great\n\n", w.Body.String())
}

func TestSubscribeAllTargets(t *testing.T) {
	hub := createDummy()
	s, _ := hub.transport.(*LocalTransport)

	go func() {
		for {
			s.RLock()
			empty := len(s.pipes) == 0
			s.RUnlock()

			if empty {
				continue
			}

			hub.transport.Write(&Update{
				Targets: map[string]struct{}{"foo": {}},
				Topics:  []string{"http://example.com/reviews/21"},
				Event:   Event{Data: "Foo", ID: "a"},
			})
			hub.transport.Write(&Update{
				Targets: map[string]struct{}{"bar": {}},
				Topics:  []string{"http://example.com/reviews/22"},
				Event:   Event{Data: "Hello World", ID: "b", Type: "test"},
			})

			time.Sleep(8 * time.Millisecond)
			hub.Stop()
			return
		}
	}()

	req := httptest.NewRequest("GET", "http://example.com/hub?topic=http://example.com/reviews/{id}", nil)
	w := httptest.NewRecorder()
	req.Header.Add("Authorization", "Bearer "+createDummyAuthorizedJWT(hub, false, []string{"random", "*"}))
	hub.SubscribeHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, ":\nid: a\ndata: Foo\n\nevent: test\nid: b\ndata: Hello World\n\n", w.Body.String())
}

func TestSendMissedEvents(t *testing.T) {
	url, _ := url.Parse("bolt://test.db")
	transport, _ := NewBoltTransport(&Options{TransportURL: url})
	defer transport.Close()
	defer os.Remove("test.db")

	hub := createAnonymousDummyWithTransport(transport)

	transport.Write(&Update{
		Topics: []string{"http://example.com/foos/a"},
		Event: Event{
			ID:   "a",
			Data: "d1",
		},
	})
	transport.Write(&Update{
		Topics: []string{"http://example.com/foos/b"},
		Event: Event{
			ID:   "b",
			Data: "d2",
		},
	})

	var wg sync.WaitGroup
	wg.Add(2)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		defer wg.Done()
		req := httptest.NewRequest("GET", "http://example.com/hub?topic=http://example.com/foos/{id}&Last-Event-ID=a", nil).WithContext(ctx)
		w := httptest.NewRecorder()
		hub.SubscribeHandler(w, req)
		assert.Equal(t, ":\nid: b\ndata: d2\n\n", w.Body.String())
	}()

	go func() {
		defer wg.Done()
		req := httptest.NewRequest("GET", "http://example.com/hub?topic=http://example.com/foos/{id}", nil).WithContext(ctx)
		req.Header.Add("Last-Event-ID", "a")
		w := httptest.NewRecorder()
		hub.SubscribeHandler(w, req)
		assert.Equal(t, ":\nid: b\ndata: d2\n\n", w.Body.String())
	}()

	for {
		transport.RLock()
		two := len(transport.pipes) == 2
		transport.RUnlock()

		if two {
			break
		}
	}

	// let time to the cursor to read history messages
	time.Sleep(1 * time.Millisecond)

	cancel()
	wg.Wait()
}

func TestSubscribeHeartbeat(t *testing.T) {
	hub := createAnonymousDummy()
	hub.options.HeartbeatInterval = 5 * time.Millisecond
	s, _ := hub.transport.(*LocalTransport)

	go func() {
		for {
			s.RLock()
			empty := len(s.pipes) == 0
			s.RUnlock()

			if empty {
				continue
			}

			hub.transport.Write(&Update{
				Topics: []string{"http://example.com/books/1"},
				Event:  Event{Data: "Hello World", ID: "b"},
			})

			time.Sleep(8 * time.Millisecond)
			hub.Stop()
			return
		}
	}()

	req := httptest.NewRequest("GET", "http://example.com/hub?topic=http://example.com/books/1&topic=http://example.com/reviews/{id}", nil)
	w := httptest.NewRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, ":\nid: b\ndata: Hello World\n\n:\n", w.Body.String())
}

func BenchmarkSubscribe(b *testing.B) {
	for n := 0; n < b.N; n++ {
		testSubscribe(1000, nil)
	}
}
