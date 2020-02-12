package hub

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
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

type responseTester struct {
	body               string
	expectedStatusCode int
	expectedBody       string
	cancel             context.CancelFunc
	t                  *testing.T
}

func (rt *responseTester) Header() http.Header {
	return http.Header{}
}

func (rt *responseTester) Write(buf []byte) (int, error) {
	rt.body += string(buf)

	if rt.body == rt.expectedBody {
		rt.cancel()
	} else if !strings.HasPrefix(rt.expectedBody, rt.body) {
		rt.t.Errorf(`Received body "%s" doesn't match expected body "%s"`, rt.body, rt.expectedBody)
		rt.cancel()
	}

	return len(buf), nil
}

func (rt *responseTester) WriteHeader(statusCode int) {
	if rt.t != nil {
		assert.Equal(rt.t, rt.expectedStatusCode, statusCode)
	}
}

func (rt *responseTester) Flush() {
}

func TestSubscribeNotAFlusher(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("GET", defaultHubURL, nil)

	assert.PanicsWithValue(t, "http.ResponseWriter must be an instance of http.Flusher", func() {
		hub.SubscribeHandler(&responseWriterMock{}, req)
	})
}

func TestSubscribeNoCookie(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("GET", defaultHubURL, nil)
	w := httptest.NewRecorder()

	hub.SubscribeHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

func TestSubscribeInvalidJWT(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("GET", defaultHubURL, nil)
	w := httptest.NewRecorder()
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: "invalid"})

	hub.SubscribeHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

func TestSubscribeUnauthorizedJWT(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("GET", defaultHubURL, nil)
	w := httptest.NewRecorder()
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyUnauthorizedJWT()})
	req.Header = http.Header{"Cookie": []string{w.Header().Get("Set-Cookie")}}

	hub.SubscribeHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

func TestSubscribeInvalidAlgJWT(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("GET", defaultHubURL, nil)
	w := httptest.NewRecorder()
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyNoneSignedJWT()})

	hub.SubscribeHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

func TestSubscribeNoTopic(t *testing.T) {
	hub := createAnonymousDummy()

	req := httptest.NewRequest("GET", defaultHubURL, nil)
	w := httptest.NewRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "Missing \"topic\" parameter.\n", w.Body.String())
}

type createPipeErrorTransport struct {
}

func (*createPipeErrorTransport) Write(update *Update) error {
	return nil
}

func (*createPipeErrorTransport) CreatePipe(fromID string) (*Pipe, error) {
	return nil, fmt.Errorf("Failed to create a pipe")
}

func (*createPipeErrorTransport) Close() error {
	return nil
}

func TestSubscribeCreatePipeError(t *testing.T) {
	hub := createDummyWithTransportAndConfig(&createPipeErrorTransport{}, viper.New())

	req := httptest.NewRequest("GET", defaultHubURL+"?topic=foo", nil)
	w := httptest.NewRecorder()

	hub.SubscribeHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusInternalServerError)+"\n", w.Body.String())
}

func testSubscribe(numberOfSubscribers int, t *testing.T) {
	hub := createAnonymousDummy()

	go func() {
		for {
			s, _ := hub.transport.(*LocalTransport)
			s.RLock()
			ready := len(s.pipes) == numberOfSubscribers
			s.RUnlock()

			// There is a problem (probably related to Logrus?) preventing the benchmark to work without this line.
			log.Info("Waiting for the subscribers...")
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

			return
		}
	}()

	var wg sync.WaitGroup
	wg.Add(numberOfSubscribers)
	for i := 0; i < numberOfSubscribers; i++ {
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithCancel(context.Background())
			req := httptest.NewRequest("GET", defaultHubURL+"?topic=http://example.com/books/1&topic=string&topic=http://example.com/reviews/{id}&topic=http://example.com/hub?topic=faulty{iri", nil).WithContext(ctx)

			w := &responseTester{
				expectedStatusCode: http.StatusOK,
				expectedBody:       ":\nid: b\ndata: Hello World\n\nid: c\ndata: Great\n\nid: d\ndata: Faulty IRI\n\nid: e\ndata: string\n\n",
				t:                  t,
				cancel:             cancel,
			}
			hub.SubscribeHandler(w, req)
		}()
	}

	wg.Wait()
	hub.Stop()
}

func TestSubscribe(t *testing.T) {
	log.SetLevel(log.ErrorLevel)
	testSubscribe(3, t)
}

func TestUnsubscribe(t *testing.T) {
	hub := createAnonymousDummy()

	s, _ := hub.transport.(*LocalTransport)
	assert.Equal(t, 0, len(s.pipes))
	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		req := httptest.NewRequest("GET", defaultHubURL+"?topic=http://example.com/books/1", nil).WithContext(ctx)
		hub.SubscribeHandler(httptest.NewRecorder(), req)
		assert.Equal(t, 1, len(s.pipes))
		for pipe := range s.pipes {
			assert.True(t, pipe.IsClosed())
		}
	}()

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
	hub.config.Set("debug", true)
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
			return
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", defaultHubURL+"?topic=http://example.com/reviews/{id}", nil).WithContext(ctx)
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(hub, subscriberRole, []string{"foo", "bar"})})

	w := &responseTester{
		expectedStatusCode: http.StatusOK,
		expectedBody:       ":\nevent: test\nid: b\ndata: Hello World\n\nretry: 1\nid: c\ndata: Great\n\n",
		t:                  t,
		cancel:             cancel,
	}

	hub.SubscribeHandler(w, req)
	hub.Stop()
}

func TestSubscriptionEvents(t *testing.T) {
	hub := createDummy()
	hub.config.Set("dispatch_subscriptions", true)
	hub.config.Set("subscriptions_include_ip", true)

	var wg sync.WaitGroup
	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())
	wg.Add(3)
	go func() {
		// Authorized to receive connection events
		defer wg.Done()
		req := httptest.NewRequest("GET", defaultHubURL+"?topic=https://mercure.rocks/subscriptions/{topic}/{connectionID}", nil).WithContext(ctx1)
		req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(hub, subscriberRole, []string{"https://mercure.rocks/targets/subscriptions"})})
		w := httptest.NewRecorder()
		hub.SubscribeHandler(w, req)

		resp := w.Result()
		defer resp.Body.Close()
		body, _ := ioutil.ReadAll(resp.Body)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		bodyContent := string(body)
		assert.Contains(t, bodyContent, `data:   "@id": "https://mercure.rocks/subscriptions/https%3A%2F%2Fexample.com/`)
		assert.Contains(t, bodyContent, `data:   "@type": "https://mercure.rocks/Subscription",`)
		assert.Contains(t, bodyContent, `data:   "topic": "https://example.com",`)
		assert.Contains(t, bodyContent, `data:   "publish": [],`)
		assert.Contains(t, bodyContent, `data:   "subscribe": []`)
		assert.Contains(t, bodyContent, `data:   "active": true,`)
		assert.Contains(t, bodyContent, `data:   "active": false,`)
		assert.Contains(t, bodyContent, `data:   "address": "`)
	}()

	go func() {
		// Not authorized to receive connection events
		defer wg.Done()
		req := httptest.NewRequest("GET", defaultHubURL+"?topic=https://mercure.rocks/subscriptions/{topic}/{connectionID}", nil).WithContext(ctx2)
		req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(hub, subscriberRole, []string{})})
		w := httptest.NewRecorder()
		hub.SubscribeHandler(w, req)

		resp := w.Result()
		defer resp.Body.Close()
		body, _ := ioutil.ReadAll(resp.Body)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, ":\n", string(body))
	}()

	go func() {
		defer wg.Done()

		s, _ := hub.transport.(*LocalTransport)
		for {
			s.RLock()
			ready := len(s.pipes) == 2
			s.RUnlock()

			log.Info("Waiting for subscriber...")
			if ready {
				break
			}
		}

		ctx, cancelRequest2 := context.WithCancel(context.Background())
		req := httptest.NewRequest("GET", defaultHubURL+"?topic=https://example.com", nil).WithContext(ctx)
		req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(hub, subscriberRole, []string{})})

		w := &responseTester{
			expectedStatusCode: http.StatusOK,
			expectedBody:       ":\n",
			t:                  t,
			cancel:             cancelRequest2,
		}
		hub.SubscribeHandler(w, req)
		time.Sleep(1 * time.Second) // TODO: find a better way to wait for the disconnection update to be dispatched
		cancel2()
		cancel1()
	}()

	wg.Wait()
	hub.Stop()
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

			return
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", defaultHubURL+"?topic=http://example.com/reviews/{id}", nil).WithContext(ctx)
	req.Header.Add("Authorization", "Bearer "+createDummyAuthorizedJWT(hub, subscriberRole, []string{"random", "*"}))

	w := &responseTester{
		expectedStatusCode: http.StatusOK,
		expectedBody:       ":\nid: a\ndata: Foo\n\nevent: test\nid: b\ndata: Hello World\n\n",
		t:                  t,
		cancel:             cancel,
	}

	hub.SubscribeHandler(w, req)
	hub.Stop()
}

func TestSendMissedEvents(t *testing.T) {
	u, _ := url.Parse("bolt://test.db")
	transport, _ := NewBoltTransport(u)
	defer transport.Close()
	defer os.Remove("test.db")

	hub := createDummyWithTransportAndConfig(transport, viper.New())

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

	go func() {
		defer wg.Done()

		ctx, cancel := context.WithCancel(context.Background())
		req := httptest.NewRequest("GET", defaultHubURL+"?topic=http://example.com/foos/{id}&Last-Event-ID=a", nil).WithContext(ctx)

		w := &responseTester{
			expectedStatusCode: http.StatusOK,
			expectedBody:       ":\nid: b\ndata: d2\n\n",
			t:                  t,
			cancel:             cancel,
		}

		hub.SubscribeHandler(w, req)
	}()

	go func() {
		defer wg.Done()

		ctx, cancel := context.WithCancel(context.Background())
		req := httptest.NewRequest("GET", defaultHubURL+"?topic=http://example.com/foos/{id}", nil).WithContext(ctx)
		req.Header.Add("Last-Event-ID", "a")

		w := &responseTester{
			expectedStatusCode: http.StatusOK,
			expectedBody:       ":\nid: b\ndata: d2\n\n",
			t:                  t,
			cancel:             cancel,
		}

		hub.SubscribeHandler(w, req)
	}()

	wg.Wait()
	hub.Stop()
}

func TestSubscribeHeartbeat(t *testing.T) {
	hub := createAnonymousDummy()
	hub.config.Set("heartbeat_interval", 5*time.Millisecond)
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

			return
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", defaultHubURL+"?topic=http://example.com/books/1&topic=http://example.com/reviews/{id}", nil).WithContext(ctx)

	w := &responseTester{
		expectedStatusCode: http.StatusOK,
		expectedBody:       ":\nid: b\ndata: Hello World\n\n:\n",
		t:                  t,
		cancel:             cancel,
	}

	hub.SubscribeHandler(w, req)
	hub.Stop()
}

func BenchmarkSubscribe(b *testing.B) {
	log.SetOutput(ioutil.Discard)
	for n := 0; n < b.N; n++ {
		testSubscribe(1000, nil)
	}
}
