package mercure

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

type responseWriterMock struct{}

func (m *responseWriterMock) Header() http.Header {
	return http.Header{}
}

func (m *responseWriterMock) Write([]byte) (int, error) {
	return 0, nil
}

func (m *responseWriterMock) WriteHeader(_ int) {
}

type responseTester struct {
	header             http.Header
	body               string
	expectedStatusCode int
	expectedBody       string
	cancel             context.CancelFunc
	t                  *testing.T
}

func (rt *responseTester) Header() http.Header {
	if rt.header == nil {
		return http.Header{}
	}

	return rt.header
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

func (rt *responseTester) SetWriteDeadline(_ time.Time) error {
	return nil
}

type subscribeRecorder struct {
	*httptest.ResponseRecorder
	writeDeadline time.Time
}

func newSubscribeRecorder() *subscribeRecorder {
	return &subscribeRecorder{ResponseRecorder: httptest.NewRecorder()}
}

func (r *subscribeRecorder) SetWriteDeadline(deadline time.Time) error {
	if deadline.After(r.writeDeadline) {
		r.writeDeadline = deadline
	}

	return nil
}

func (r *subscribeRecorder) Write(buf []byte) (int, error) {
	if time.Now().After(r.writeDeadline) {
		return 0, os.ErrDeadlineExceeded
	}

	return r.ResponseRecorder.Write(buf)
}

func (r *subscribeRecorder) WriteString(str string) (int, error) {
	if time.Now().After(r.writeDeadline) {
		return 0, os.ErrDeadlineExceeded
	}

	return r.ResponseRecorder.WriteString(str)
}

func (r *subscribeRecorder) FlushError() error {
	if time.Now().After(r.writeDeadline) {
		return os.ErrDeadlineExceeded
	}

	r.ResponseRecorder.Flush()

	return nil
}

func TestSubscribeNotAFlusher(t *testing.T) {
	hub := createAnonymousDummy()

	go func() {
		s := hub.transport.(*LocalTransport)
		var ready bool

		for !ready {
			s.RLock()
			ready = s.subscribers.Len() != 0
			s.RUnlock()
		}

		hub.transport.Dispatch(&Update{
			Topics: []string{"http://example.com/foo"},
			Event:  Event{Data: "Hello World"},
		})
	}()

	assert.Panics(t, func() {
		hub.SubscribeHandler(
			&responseWriterMock{},
			httptest.NewRequest(http.MethodGet, defaultHubURL+"?topic=http://example.com/foo", nil),
		)
	})
}

func TestSubscribeNoCookie(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest(http.MethodGet, defaultHubURL, nil)
	w := httptest.NewRecorder()

	hub.SubscribeHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

func TestSubscribeInvalidJWT(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest(http.MethodGet, defaultHubURL, nil)
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

	req := httptest.NewRequest(http.MethodGet, defaultHubURL, nil)
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

	req := httptest.NewRequest(http.MethodGet, defaultHubURL, nil)
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

	req := httptest.NewRequest(http.MethodGet, defaultHubURL, nil)
	w := httptest.NewRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "Missing \"topic\" parameter.\n", w.Body.String())
}

var errFailedToAddSubscriber = errors.New("failed to add a subscriber")

type addSubscriberErrorTransport struct{}

func (*addSubscriberErrorTransport) Dispatch(*Update) error {
	return nil
}

func (*addSubscriberErrorTransport) AddSubscriber(*Subscriber) error {
	return errFailedToAddSubscriber
}

func (*addSubscriberErrorTransport) RemoveSubscriber(*Subscriber) error {
	return nil
}

func (*addSubscriberErrorTransport) GetSubscribers() (string, []*Subscriber, error) {
	return "", []*Subscriber{}, nil
}

func (*addSubscriberErrorTransport) Close() error {
	return nil
}

func TestSubscribeAddSubscriberError(t *testing.T) {
	hub := createAnonymousDummy(WithTransport(&addSubscriberErrorTransport{}))

	req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?topic=foo", nil)
	w := httptest.NewRecorder()

	hub.SubscribeHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusServiceUnavailable)+"\n", w.Body.String())
}

func testSubscribe(h interface{ Helper() }, numberOfSubscribers int) {
	h.Helper()

	hub := createAnonymousDummy()

	go func() {
		s := hub.transport.(*LocalTransport)
		var ready bool

		for !ready {
			s.RLock()
			ready = s.subscribers.Len() == numberOfSubscribers
			s.RUnlock()
		}

		hub.transport.Dispatch(&Update{
			Topics: []string{"http://example.com/not-subscribed"},
			Event:  Event{Data: "Hello World", ID: "a"},
		})
		hub.transport.Dispatch(&Update{
			Topics: []string{"http://example.com/books/1"},
			Event:  Event{Data: "Hello World", ID: "b"},
		})
		hub.transport.Dispatch(&Update{
			Topics: []string{"http://example.com/reviews/22"},
			Event:  Event{Data: "Great", ID: "c"},
		})
		hub.transport.Dispatch(&Update{
			Topics: []string{"http://example.com/hub?topic=faulty{iri"},
			Event:  Event{Data: "Faulty IRI", ID: "d"},
		})
		hub.transport.Dispatch(&Update{
			Topics: []string{"string"},
			Event:  Event{Data: "string", ID: "e"},
		})
	}()

	t, _ := h.(*testing.T)
	var wg sync.WaitGroup
	wg.Add(numberOfSubscribers)
	for i := 0; i < numberOfSubscribers; i++ {
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithCancel(context.Background())
			req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?topic=http://example.com/books/1&topic=string&topic=http://example.com/reviews/{id}&topic=http://example.com/hub?topic=faulty{iri", nil).WithContext(ctx)

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
}

func TestSubscribe(t *testing.T) {
	testSubscribe(t, 3)
}

func testSubscribeLogs(t *testing.T, hub *Hub, payload interface{}) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?topic=http://example.com/reviews/{id}", nil).WithContext(ctx)
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWTWithPayload(roleSubscriber, []string{"http://example.com/reviews/22"}, payload)})

	w := &responseTester{
		expectedStatusCode: http.StatusOK,
		expectedBody:       ":\n",
		t:                  t,
		cancel:             cancel,
	}

	hub.SubscribeHandler(w, req)
}

func TestSubscribeWithLogLevelDebug(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	payload := map[string]interface{}{
		"bar": "baz",
		"foo": "bar",
	}

	testSubscribeLogs(t, createDummy(
		WithLogger(zap.New(core)),
	), payload)

	assert.Equal(t, 1, logs.FilterMessage("New subscriber").FilterField(
		zap.Reflect("payload", payload)).Len(),
	)
}

func TestSubscribeLogLevelInfo(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	payload := map[string]interface{}{
		"bar": "baz",
		"foo": "bar",
	}
	testSubscribeLogs(t, createDummy(
		WithLogger(zap.New(core)),
	), payload)

	assert.Equal(t, 0, logs.FilterMessage("New subscriber").FilterFieldKey("payload").Len())
}

func TestSubscribeLogAnonymousSubscriber(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)

	h := createAnonymousDummy(WithLogger(zap.New(core)))

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?topic=http://example.com/", nil).WithContext(ctx)

	w := &responseTester{
		expectedStatusCode: http.StatusOK,
		expectedBody:       ":\n",
		t:                  t,
		cancel:             cancel,
	}

	h.SubscribeHandler(w, req)

	assert.Equal(t, 0, logs.FilterMessage("New subscriber").FilterFieldKey("payload").Len())
}

func TestUnsubscribe(t *testing.T) {
	hub := createAnonymousDummy()

	s, _ := hub.transport.(*LocalTransport)
	assert.Equal(t, 0, s.subscribers.Len())
	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?topic=http://example.com/books/1", nil).WithContext(ctx)
		hub.SubscribeHandler(newSubscribeRecorder(), req)
		assert.Equal(t, 0, s.subscribers.Len())
		s.subscribers.Walk(0, func(s *Subscriber) bool {
			_, ok := <-s.out
			assert.False(t, ok)

			return true
		})
	}()

	for {
		s.RLock()
		notEmpty := s.subscribers.Len() != 0
		s.RUnlock()
		if notEmpty {
			break
		}
	}

	cancel()
	wg.Wait()
}

func TestSubscribePrivate(t *testing.T) {
	hub := createDummy()
	s, _ := hub.transport.(*LocalTransport)

	go func() {
		for {
			s.RLock()
			empty := s.subscribers.Len() == 0
			s.RUnlock()

			if empty {
				continue
			}

			hub.transport.Dispatch(&Update{
				Topics:  []string{"http://example.com/reviews/21"},
				Event:   Event{Data: "Foo", ID: "a"},
				Private: true,
			})
			hub.transport.Dispatch(&Update{
				Topics:  []string{"http://example.com/reviews/22"},
				Event:   Event{Data: "Hello World", ID: "b", Type: "test"},
				Private: true,
			})
			hub.transport.Dispatch(&Update{
				Topics:  []string{"http://example.com/reviews/23"},
				Event:   Event{Data: "Great", ID: "c", Retry: 1},
				Private: true,
			})

			return
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?topic=http://example.com/reviews/{id}", nil).WithContext(ctx)
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(roleSubscriber, []string{"http://example.com/reviews/22", "http://example.com/reviews/23"})})

	w := &responseTester{
		expectedStatusCode: http.StatusOK,
		expectedBody:       ":\nevent: test\nid: b\ndata: Hello World\n\nretry: 1\nid: c\ndata: Great\n\n",
		t:                  t,
		cancel:             cancel,
	}

	hub.SubscribeHandler(w, req)
}

func TestSubscriptionEvents(t *testing.T) {
	hub := createDummy(WithSubscriptions())

	var wg sync.WaitGroup
	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())
	wg.Add(3)
	go func() {
		// Authorized to receive connection events
		defer wg.Done()
		req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?topic=/.well-known/mercure/subscriptions/{topic}/{subscriber}", nil).WithContext(ctx1)
		req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(roleSubscriber, []string{"/.well-known/mercure/subscriptions/{topic}/{subscriber}"})})
		w := newSubscribeRecorder()
		hub.SubscribeHandler(w, req)

		resp := w.Result()
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		bodyContent := string(body)
		assert.Contains(t, bodyContent, `data:   "@context": "https://mercure.rocks/",`)
		assert.Regexp(t, `(?m)^data:   "id": "/\.well-known/mercure/subscriptions/https%3A%2F%2Fexample\.com/.*,$`, bodyContent)
		assert.Contains(t, bodyContent, `data:   "type": "Subscription",`)
		assert.Contains(t, bodyContent, `data:   "subscriber": "urn:uuid:`)
		assert.Contains(t, bodyContent, `data:   "topic": "https://example.com",`)
		assert.Contains(t, bodyContent, `data:   "active": true,`)
		assert.Contains(t, bodyContent, `data:   "active": false,`)
		assert.Contains(t, bodyContent, `data:   "payload": {`)
		assert.Contains(t, bodyContent, `data:     "foo": "bar"`)
	}()

	go func() {
		// Not authorized to receive connection events
		defer wg.Done()
		req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?topic=/.well-known/mercure/subscriptions/{topicSelector}/{subscriber}", nil).WithContext(ctx2)
		req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(roleSubscriber, []string{})})
		w := newSubscribeRecorder()
		hub.SubscribeHandler(w, req)

		resp := w.Result()
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "", string(body))
	}()

	go func() {
		defer wg.Done()

		for {
			_, s, _ := hub.transport.(TransportSubscribers).GetSubscribers()
			if len(s) == 2 {
				break
			}
		}

		ctx, cancelRequest2 := context.WithCancel(context.Background())
		req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?topic=https://example.com", nil).WithContext(ctx)
		req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(roleSubscriber, []string{})})

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
}

func TestSubscribeAll(t *testing.T) {
	hub := createDummy()
	s, _ := hub.transport.(*LocalTransport)

	go func() {
		for {
			s.RLock()
			empty := s.subscribers.Len() == 0
			s.RUnlock()

			if empty {
				continue
			}

			hub.transport.Dispatch(&Update{
				Topics:  []string{"http://example.com/reviews/21"},
				Event:   Event{Data: "Foo", ID: "a"},
				Private: true,
			})
			hub.transport.Dispatch(&Update{
				Topics:  []string{"http://example.com/reviews/22"},
				Event:   Event{Data: "Hello World", ID: "b", Type: "test"},
				Private: true,
			})

			return
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?topic=http://example.com/reviews/{id}", nil).WithContext(ctx)
	req.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(roleSubscriber, []string{"random", "*"}))

	w := &responseTester{
		expectedStatusCode: http.StatusOK,
		expectedBody:       ":\nid: a\ndata: Foo\n\nevent: test\nid: b\ndata: Hello World\n\n",
		t:                  t,
		cancel:             cancel,
	}

	hub.SubscribeHandler(w, req)
}

func TestSendMissedEvents(t *testing.T) {
	bt := createBoltTransport("bolt://test.db")
	hub := createAnonymousDummy(WithLogger(bt.logger), WithTransport(bt), WithProtocolVersionCompatibility(7))
	transport := hub.transport.(*BoltTransport)
	defer transport.Close()
	defer os.Remove("test.db")

	transport.Dispatch(&Update{
		Topics: []string{"http://example.com/foos/a"},
		Event: Event{
			ID:   "a",
			Data: "d1",
		},
	})
	transport.Dispatch(&Update{
		Topics: []string{"http://example.com/foos/b"},
		Event: Event{
			ID:   "b",
			Data: "d2",
		},
	})

	var wg sync.WaitGroup
	wg.Add(3)

	// Using deprecated 'Last-Event-ID' query parameter
	go func() {
		defer wg.Done()

		ctx, cancel := context.WithCancel(context.Background())
		req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?topic=http://example.com/foos/{id}&Last-Event-ID=a", nil).WithContext(ctx)

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
		req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?topic=http://example.com/foos/{id}&lastEventID=a", nil).WithContext(ctx)

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
		req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?topic=http://example.com/foos/{id}", nil).WithContext(ctx)
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
}

func TestSendAllEvents(t *testing.T) {
	bt := createBoltTransport("bolt://test.db")
	hub := createAnonymousDummy(WithLogger(bt.logger), WithTransport(bt))
	transport := hub.transport.(*BoltTransport)
	defer transport.Close()
	defer os.Remove("test.db")

	transport.Dispatch(&Update{
		Topics: []string{"http://example.com/foos/a"},
		Event: Event{
			ID:   "a",
			Data: "d1",
		},
	})
	transport.Dispatch(&Update{
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
		req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?topic=http://example.com/foos/{id}&lastEventID="+EarliestLastEventID, nil).WithContext(ctx)

		w := &responseTester{
			header:             http.Header{},
			expectedStatusCode: http.StatusOK,
			expectedBody:       ":\nid: a\ndata: d1\n\nid: b\ndata: d2\n\n",
			t:                  t,
			cancel:             cancel,
		}

		hub.SubscribeHandler(w, req)
	}()

	go func() {
		defer wg.Done()

		ctx, cancel := context.WithCancel(context.Background())
		req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?topic=http://example.com/foos/{id}", nil).WithContext(ctx)
		req.Header.Add("Last-Event-ID", EarliestLastEventID)

		w := &responseTester{
			header:             http.Header{},
			expectedStatusCode: http.StatusOK,
			expectedBody:       ":\nid: a\ndata: d1\n\nid: b\ndata: d2\n\n",
			t:                  t,
			cancel:             cancel,
		}

		hub.SubscribeHandler(w, req)
	}()

	wg.Wait()
}

func TestUnknownLastEventID(t *testing.T) {
	bt := createBoltTransport("bolt://test.db")
	hub := createAnonymousDummy(WithLogger(bt.logger), WithTransport(bt))
	transport := hub.transport.(*BoltTransport)
	defer transport.Close()
	defer os.Remove("test.db")

	transport.Dispatch(&Update{
		Topics: []string{"http://example.com/foos/a"},
		Event: Event{
			ID:   "a",
			Data: "d1",
		},
	})

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()

		ctx, cancel := context.WithCancel(context.Background())
		req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?topic=http://example.com/foos/{id}&lastEventID=unknown", nil).WithContext(ctx)

		w := &responseTester{
			header:             http.Header{},
			expectedStatusCode: http.StatusOK,
			expectedBody:       ":\nid: b\ndata: d2\n\n",
			t:                  t,
			cancel:             cancel,
		}

		hub.SubscribeHandler(w, req)
		assert.Equal(t, "a", w.Header().Get("Last-Event-ID"))
	}()

	go func() {
		defer wg.Done()

		ctx, cancel := context.WithCancel(context.Background())
		req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?topic=http://example.com/foos/{id}", nil).WithContext(ctx)
		req.Header.Add("Last-Event-ID", "unknown")

		w := &responseTester{
			header:             http.Header{},
			expectedStatusCode: http.StatusOK,
			expectedBody:       ":\nid: b\ndata: d2\n\n",
			t:                  t,
			cancel:             cancel,
		}

		hub.SubscribeHandler(w, req)
		assert.Equal(t, "a", w.Header().Get("Last-Event-ID"))
	}()

	for {
		transport.RLock()
		done := transport.subscribers.Len() == 2
		transport.RUnlock()

		if done {
			break
		}
	}

	transport.Dispatch(&Update{
		Topics: []string{"http://example.com/foos/b"},
		Event: Event{
			ID:   "b",
			Data: "d2",
		},
	})

	wg.Wait()
}

func TestUnknownLastEventIDEmptyHistory(t *testing.T) {
	bt := createBoltTransport("bolt://test.db")
	hub := createAnonymousDummy(WithLogger(bt.logger), WithTransport(bt))
	transport := hub.transport.(*BoltTransport)
	defer transport.Close()
	defer os.Remove("test.db")

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()

		ctx, cancel := context.WithCancel(context.Background())
		req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?topic=http://example.com/foos/{id}&lastEventID=unknown", nil).WithContext(ctx)

		w := &responseTester{
			header:             http.Header{},
			expectedStatusCode: http.StatusOK,
			expectedBody:       ":\nid: b\ndata: d2\n\n",
			t:                  t,
			cancel:             cancel,
		}

		hub.SubscribeHandler(w, req)
		assert.Equal(t, EarliestLastEventID, w.Header().Get("Last-Event-ID"))
	}()

	go func() {
		defer wg.Done()

		ctx, cancel := context.WithCancel(context.Background())
		req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?topic=http://example.com/foos/{id}", nil).WithContext(ctx)
		req.Header.Add("Last-Event-ID", "unknown")

		w := &responseTester{
			header:             http.Header{},
			expectedStatusCode: http.StatusOK,
			expectedBody:       ":\nid: b\ndata: d2\n\n",
			t:                  t,
			cancel:             cancel,
		}

		hub.SubscribeHandler(w, req)
		assert.Equal(t, EarliestLastEventID, w.Header().Get("Last-Event-ID"))
	}()

	for {
		transport.RLock()
		done := transport.subscribers.Len() == 2
		transport.RUnlock()

		if done {
			break
		}
	}

	transport.Dispatch(&Update{
		Topics: []string{"http://example.com/foos/b"},
		Event: Event{
			ID:   "b",
			Data: "d2",
		},
	})

	wg.Wait()
}

func TestSubscribeHeartbeat(t *testing.T) {
	hub := createAnonymousDummy(WithHeartbeat(5 * time.Millisecond))
	s, _ := hub.transport.(*LocalTransport)

	go func() {
		for {
			s.RLock()
			empty := s.subscribers.Len() == 0
			s.RUnlock()

			if empty {
				continue
			}

			hub.transport.Dispatch(&Update{
				Topics: []string{"http://example.com/books/1"},
				Event:  Event{Data: "Hello World", ID: "b"},
			})

			return
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?topic=http://example.com/books/1&topic=http://example.com/reviews/{id}", nil).WithContext(ctx)

	w := &responseTester{
		expectedStatusCode: http.StatusOK,
		expectedBody:       ":\nid: b\ndata: Hello World\n\n:\n",
		t:                  t,
		cancel:             cancel,
	}

	hub.SubscribeHandler(w, req)
}

func TestSubscribeExpires(t *testing.T) {
	hub := createAnonymousDummy(WithWriteTimeout(0), WithDispatchTimeout(0), WithHeartbeat(500*time.Millisecond))
	token := jwt.New(jwt.SigningMethodHS256)

	token.Claims = &claims{
		Mercure: mercureClaim{
			Subscribe: []string{"*"},
		},
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Second))},
	}

	jwt, err := token.SignedString([]byte("subscriber"))
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?topic=foo", nil)
	req.Header.Add("Authorization", bearerPrefix+jwt)

	w := newSubscribeRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)
	assert.True(t, time.Now().After(token.Claims.(*claims).ExpiresAt.Time))
}

func BenchmarkSubscribe(b *testing.B) {
	for n := 0; n < b.N; n++ {
		testSubscribe(b, 1000)
	}
}
