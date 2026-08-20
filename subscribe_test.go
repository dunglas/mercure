package mercure

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	tb                 testing.TB
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
		defer rt.cancel()

		mess := fmt.Sprintf(`Received body "%s" doesn't match expected body "%s"`, rt.body, rt.expectedBody)
		if rt.tb == nil {
			panic(mess)
		}

		rt.tb.Error(mess)
	}

	return len(buf), nil
}

func (rt *responseTester) WriteHeader(statusCode int) {
	if rt.tb != nil {
		assert.Equal(rt.tb, rt.expectedStatusCode, statusCode)
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

	return r.WriteString(str)
}

func (r *subscribeRecorder) FlushError() error {
	if time.Now().After(r.writeDeadline) {
		return os.ErrDeadlineExceeded
	}

	r.Flush()

	return nil
}

func TestSubscribeNotAFlusher(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t)

	go func() {
		s := hub.transport.(*LocalTransport)

		var ready bool

		for !ready {
			s.RLock()
			ready = s.subscribers.Len() != 0
			s.RUnlock()
		}

		_ = hub.transport.Dispatch(t.Context(), &Update{
			Topics: []string{"https://example.com/foo"},
			Event:  Event{Data: "Hello World"},
		})
	}()

	assert.Panics(t, func() {
		hub.SubscribeHandler(
			&responseWriterMock{},
			httptest.NewRequest(http.MethodGet, defaultHubURL+"?match=https://example.com/foo", nil),
		)
	})
}

func TestSubscribeNoCookie(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)

	req := httptest.NewRequest(http.MethodGet, defaultHubURL, nil)
	w := httptest.NewRecorder()

	hub.SubscribeHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() {
		require.NoError(t, resp.Body.Close())
	})

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

func TestSubscribeInvalidJWT(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)

	req := httptest.NewRequest(http.MethodGet, defaultHubURL, nil)
	w := httptest.NewRecorder()

	req.AddCookie(&http.Cookie{Name: defaultCookieName, Value: "invalid"})

	hub.SubscribeHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() {
		require.NoError(t, resp.Body.Close())
	})

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

func TestSubscribeUnauthorizedJWT(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)

	req := httptest.NewRequest(http.MethodGet, defaultHubURL, nil)
	w := httptest.NewRecorder()

	req.AddCookie(&http.Cookie{Name: defaultCookieName, Value: createDummyUnauthorizedJWT()})
	req.Header = http.Header{"Cookie": []string{w.Header().Get("Set-Cookie")}}

	hub.SubscribeHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() {
		require.NoError(t, resp.Body.Close())
	})

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

func TestSubscribeInvalidAlgJWT(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)

	req := httptest.NewRequest(http.MethodGet, defaultHubURL, nil)
	w := httptest.NewRecorder()

	req.AddCookie(&http.Cookie{Name: defaultCookieName, Value: createDummyNoneSignedJWT()})

	hub.SubscribeHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() {
		require.NoError(t, resp.Body.Close())
	})

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

// TestSubscribeJWTAlgorithmsPinned verifies the algorithm allowlist is enforced
// on the keyfunc (JWKS-style) path: a token whose alg is outside the allowlist
// is rejected at parse time, regardless of its signature.
func TestSubscribeJWTAlgorithmsPinned(t *testing.T) {
	t.Parallel()

	// A keyfunc returning the HMAC secret for any token, like a JWKS-backed
	// keyfunc that does not by itself pin the algorithm.
	kf := func(*jwt.Token) (any, error) { return []byte("subscriber"), nil }

	tms, err := NewTopicMatcherStore(0)
	require.NoError(t, err)

	hub, err := NewHub(t.Context(),
		WithAnonymous(),
		WithResourceIdentifier(testResourceIdentifier),
		WithIssuers([]Issuer{{
			Identifier: testIssuer,
			Subscriber: KeyFunc{Keyfunc: kf, Algorithms: []string{jwt.SigningMethodRS256.Name}},
		}}),
		WithTopicMatcherStore(tms),
	)
	require.NoError(t, err)

	// HS256 token: outside the RS256 allowlist.
	token := mintAccessToken([]byte("subscriber"), testResourceIdentifier, []authorizationDetail{{
		Type: authorizationDetailTypeMercure, Actions: []mercureAction{actionSubscribe},
		Topics: []detailTopic{{TopicMatcher{MatcherTypeExact, "https://example.com/foo"}}},
	}})

	req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match=https://example.com/foo", nil)
	req.Header.Add("Authorization", bearerPrefix+token)

	w := httptest.NewRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() {
		require.NoError(t, resp.Body.Close())
	})

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestSubscribeNoTopic(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t)

	req := httptest.NewRequest(http.MethodGet, defaultHubURL, nil)
	w := httptest.NewRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() {
		require.NoError(t, resp.Body.Close())
	})

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "missing \"match\" subscription parameter\n", w.Body.String())
}

func TestSubscribeTooManyTopics(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t)

	q := url.Values{}
	for i := 0; i <= maxMatcherCount; i++ {
		q.Add("match", "https://example.com/foo")
	}

	req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?"+q.Encode(), nil)
	w := httptest.NewRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() {
		require.NoError(t, resp.Body.Close())
	})

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestSubscribeTooManyClaimMatchers(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)

	scope := make([]string, maxClaimMatchers+1)
	for i := range scope {
		scope[i] = "https://example.com/foo"
	}

	req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match=https://example.com/foo", nil)
	req.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(roleSubscriber, scope))

	w := httptest.NewRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() {
		require.NoError(t, resp.Body.Close())
	})

	// Too many topics in a single authorization detail → invalid_token.
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

var errFailedToAddSubscriber = errors.New("failed to add a subscriber")

type addSubscriberErrorTransport struct{}

func (*addSubscriberErrorTransport) Dispatch(_ context.Context, _ *Update) error {
	return nil
}

func (*addSubscriberErrorTransport) AddSubscriber(_ context.Context, _ *LocalSubscriber) error {
	return errFailedToAddSubscriber
}

func (*addSubscriberErrorTransport) RemoveSubscriber(_ context.Context, _ *LocalSubscriber) error {
	return nil
}

func (*addSubscriberErrorTransport) GetSubscribers(_ context.Context) (string, []*LocalSubscriber, error) {
	return "", []*LocalSubscriber{}, nil
}

func (*addSubscriberErrorTransport) Close(_ context.Context) error {
	return nil
}

func TestSubscribeAddSubscriberError(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t, WithTransport(&addSubscriberErrorTransport{}))

	req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match=foo", nil)
	w := httptest.NewRecorder()

	hub.SubscribeHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() {
		require.NoError(t, resp.Body.Close())
	})

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusServiceUnavailable)+"\n", w.Body.String())
}

func TestSubscribeQueryMethod(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t)
	ctx := t.Context()

	go func() {
		s := hub.transport.(*LocalTransport)

		var ready bool

		for !ready {
			s.RLock()
			ready = s.subscribers.Len() == 1
			s.RUnlock()
		}

		_ = hub.transport.Dispatch(ctx, &Update{
			Topics: []string{"https://example.com/books/1"},
			Event:  Event{Data: "Hello World", ID: "b"},
		})
	}()

	reqCtx, cancel := context.WithCancel(t.Context())
	// Topics travel in the QUERY request body instead of the URL.
	body := url.Values{"match": {"https://example.com/books/1"}}.Encode()
	req := httptest.NewRequest(methodQuery, defaultHubURL, strings.NewReader(body)).WithContext(reqCtx)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := &responseTester{
		expectedStatusCode: http.StatusOK,
		expectedBody:       ":\nid: b\ndata: Hello World\n\n",
		tb:                 t,
		cancel:             cancel,
	}
	hub.SubscribeHandler(w, req)
}

func subscribe(tb testing.TB, numberOfSubscribers int) {
	tb.Helper()

	hub := createAnonymousDummy(tb)
	ctx := tb.Context()

	go func() {
		s := hub.transport.(*LocalTransport)

		var ready bool

		for !ready {
			s.RLock()
			ready = s.subscribers.Len() == numberOfSubscribers
			s.RUnlock()
		}

		_ = hub.transport.Dispatch(ctx, &Update{
			Topics: []string{"https://example.com/not-subscribed"},
			Event:  Event{Data: "Hello World", ID: "a"},
		})
		_ = hub.transport.Dispatch(ctx, &Update{
			Topics: []string{"https://example.com/books/1"},
			Event:  Event{Data: "Hello World", ID: "b"},
		})
		_ = hub.transport.Dispatch(ctx, &Update{
			Topics: []string{"https://example.com/reviews/22"},
			Event:  Event{Data: "Great", ID: "c"},
		})
		_ = hub.transport.Dispatch(ctx, &Update{
			Topics: []string{"https://example.com/hub?topic=faulty{iri"},
			Event:  Event{Data: "Faulty IRI", ID: "d"},
		})
		_ = hub.transport.Dispatch(ctx, &Update{
			Topics: []string{"string"},
			Event:  Event{Data: "string", ID: "e"},
		})
	}()

	var wg sync.WaitGroup

	for range numberOfSubscribers {
		wg.Go(func() {
			ctx, cancel := context.WithCancel(tb.Context())
			req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match=https://example.com/books/1&match=string&match_urlpattern=https://example.com/reviews/:id&match=https://example.com/hub?topic=faulty{iri", nil).WithContext(ctx)

			w := &responseTester{
				expectedStatusCode: http.StatusOK,
				expectedBody:       ":\nid: b\ndata: Hello World\n\nid: c\ndata: Great\n\nid: d\ndata: Faulty IRI\n\nid: e\ndata: string\n\n",
				tb:                 tb,
				cancel:             cancel,
			}
			hub.SubscribeHandler(w, req)
		})
	}

	wg.Wait()
}

func TestSubscribe(t *testing.T) {
	t.Parallel()

	subscribe(t, 3)
}

func testSubscribeLogs(t *testing.T, hub *Hub, payload any) {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match_urlpattern=https://example.com/reviews/:id", nil).WithContext(ctx)
	req.AddCookie(&http.Cookie{Name: defaultCookieName, Value: createDummySubscriberJWTWithDetails(t, payload, TopicMatcher{Type: MatcherTypeURLPattern, Pattern: "https://example.com/reviews/:id"})})

	w := &responseTester{
		expectedStatusCode: http.StatusOK,
		expectedBody:       ":\n",
		tb:                 t,
		cancel:             cancel,
	}

	hub.SubscribeHandler(w, req)
}

func TestSubscribeWithLogLevelDebug(t *testing.T) {
	t.Parallel()

	payload := map[string]any{
		"bar": "baz",
		"foo": "bar",
	}

	var buf bytes.Buffer

	opts := slog.HandlerOptions{Level: slog.LevelDebug}
	logger := slog.New(slog.NewTextHandler(&buf, &opts))

	testSubscribeLogs(t, createDummy(
		t,
		WithLogger(logger),
	), payload)

	assert.Contains(t, buf.String(), "baz")
}

func TestSubscribeLogLevelInfo(t *testing.T) {
	t.Parallel()

	payload := map[string]any{
		"bar": "baz",
		"foo": "bar",
	}

	var buf bytes.Buffer

	opts := slog.HandlerOptions{Level: slog.LevelInfo}
	logger := slog.New(slog.NewTextHandler(&buf, &opts))

	testSubscribeLogs(t, createDummy(
		t,
		WithLogger(logger),
	), payload)

	assert.NotContains(t, buf.String(), "baz")
}

func TestSubscribeLogAnonymousSubscriber(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	h := createAnonymousDummy(t, WithLogger(logger))

	ctx, cancel := context.WithCancel(t.Context())
	req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match=https://example.com/", nil).WithContext(ctx)

	w := &responseTester{
		expectedStatusCode: http.StatusOK,
		expectedBody:       ":\n",
		tb:                 t,
		cancel:             cancel,
	}

	h.SubscribeHandler(w, req)

	assert.NotContains(t, buf.String(), "payload")
}

func TestUnsubscribe(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		hub := createAnonymousDummy(t)

		s, _ := hub.transport.(*LocalTransport)
		assert.Equal(t, 0, s.subscribers.Len())
		ctx, cancel := context.WithCancel(t.Context())

		go func() {
			req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match=https://example.com/books/1", nil).WithContext(ctx)
			hub.SubscribeHandler(newSubscribeRecorder(), req)
			assert.Equal(t, 0, s.subscribers.Len())
			s.subscribers.Walk(0, func(s *LocalSubscriber) bool {
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
		synctest.Wait()
	})
}

func TestSubscribePrivate(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)
	s, _ := hub.transport.(*LocalTransport)
	ctx := t.Context()

	go func() {
		for {
			s.RLock()
			empty := s.subscribers.Len() == 0
			s.RUnlock()

			if empty {
				continue
			}

			_ = hub.transport.Dispatch(ctx, &Update{
				Topics:  []string{"https://example.com/reviews/21"},
				Event:   Event{Data: "Foo", ID: "a"},
				Private: true,
			})
			_ = hub.transport.Dispatch(ctx, &Update{
				Topics:  []string{"https://example.com/reviews/22"},
				Event:   Event{Data: "Hello World", ID: "b", Type: "test"},
				Private: true,
			})
			_ = hub.transport.Dispatch(ctx, &Update{
				Topics:  []string{"https://example.com/reviews/23"},
				Event:   Event{Data: "Great", ID: "c", Retry: 1},
				Private: true,
			})

			return
		}
	}()

	ctx, cancel := context.WithCancel(t.Context())
	req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match_urlpattern=https://example.com/reviews/:id", nil).WithContext(ctx)
	req.AddCookie(&http.Cookie{Name: defaultCookieName, Value: createDummyAuthorizedJWT(roleSubscriber, []string{"https://example.com/reviews/22", "https://example.com/reviews/23"})})

	w := &responseTester{
		expectedStatusCode: http.StatusOK,
		expectedBody:       ":\nevent: test\nid: b\ndata: Hello World\n\nretry: 1\nid: c\ndata: Great\n\n",
		tb:                 t,
		cancel:             cancel,
	}

	hub.SubscribeHandler(w, req)
}

func TestSubscriptionEvents(t *testing.T) {
	t.Parallel()

	hub := createDummy(t, WithSubscriptions())

	ctx1, cancel1 := context.WithCancel(t.Context())
	t.Cleanup(cancel1)

	ctx2, cancel2 := context.WithCancel(t.Context())
	t.Cleanup(cancel2)

	var wg sync.WaitGroup

	wg.Go(func() {
		// Authorized to receive connection events
		req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match_urlpattern=/.well-known/mercure/subscriptions/*", nil).WithContext(ctx1)
		req.AddCookie(&http.Cookie{Name: defaultCookieName, Value: createDummySubscriberJWTWithDetails(t, struct {
			Foo string `json:"foo"`
		}{Foo: "bar"}, TopicMatcher{Type: MatcherTypeURLPattern, Pattern: "/.well-known/mercure/subscriptions/*"})})

		w := newSubscribeRecorder()
		hub.SubscribeHandler(w, req)

		resp := w.Result()

		t.Cleanup(func() {
			_ = resp.Body.Close()
		})

		body, _ := io.ReadAll(resp.Body)

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		bodyContent := string(body)
		assert.Contains(t, bodyContent, "event: mercure\n")
		assert.Regexp(t, `(?m)^data:   "id": "/\.well-known/mercure/subscriptions/exact/https%3A%2F%2Fexample\.com/.*,$`, bodyContent)
		assert.Contains(t, bodyContent, `data:   "type": "subscription",`)
		assert.Contains(t, bodyContent, `data:   "subscriber": "urn:uuid:`)
		assert.Contains(t, bodyContent, `data:   "match": "https://example.com",`)
		assert.Contains(t, bodyContent, `data:   "match_type": "exact",`)
		assert.Contains(t, bodyContent, `data:   "active": true,`)
		assert.Contains(t, bodyContent, `data:   "active": false,`)
		assert.Contains(t, bodyContent, `data:   "payload": {`)
		assert.Contains(t, bodyContent, `data:     "foo": "bar"`)
	})

	wg.Go(func() {
		// Not authorized to receive connection events
		req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match_urlpattern=/.well-known/mercure/subscriptions/:match_type/:match/:subscriber", nil).WithContext(ctx2)
		req.AddCookie(&http.Cookie{Name: defaultCookieName, Value: createDummyAuthorizedJWT(roleSubscriber, []string{})})

		w := newSubscribeRecorder()
		hub.SubscribeHandler(w, req)

		resp := w.Result()

		t.Cleanup(func() {
			_ = resp.Body.Close()
		})

		body, _ := io.ReadAll(resp.Body)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Empty(t, string(body))
	})

	wg.Go(func() {
		ctx := t.Context()

		for {
			_, s, _ := hub.transport.(TransportSubscribers).GetSubscribers(ctx)
			if len(s) == 2 {
				break
			}
		}

		ctx, cancelRequest2 := context.WithCancel(ctx)
		req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match=https://example.com", nil).WithContext(ctx)
		req.AddCookie(&http.Cookie{Name: defaultCookieName, Value: createDummyAuthorizedJWT(roleSubscriber, []string{"https://example.com"})})

		w := &responseTester{
			expectedStatusCode: http.StatusOK,
			expectedBody:       ":\n",
			tb:                 t,
			cancel:             cancelRequest2,
		}
		hub.SubscribeHandler(w, req)
		time.Sleep(1 * time.Second) // TODO: find a better way to wait for the disconnection update to be dispatched
		cancel2()
		cancel1()
	})

	wg.Wait()
}

func TestSubscribeAll(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)
	s, _ := hub.transport.(*LocalTransport)
	ctx := t.Context()

	go func() {
		for {
			s.RLock()
			empty := s.subscribers.Len() == 0
			s.RUnlock()

			if empty {
				continue
			}

			_ = hub.transport.Dispatch(ctx, &Update{
				Topics:  []string{"https://example.com/reviews/21"},
				Event:   Event{Data: "Foo", ID: "a"},
				Private: true,
			})
			_ = hub.transport.Dispatch(ctx, &Update{
				Topics:  []string{"https://example.com/reviews/22"},
				Event:   Event{Data: "Hello World", ID: "b", Type: "test"},
				Private: true,
			})

			return
		}
	}()

	ctx, cancel := context.WithCancel(t.Context())
	req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match_urlpattern=https://example.com/reviews/:id", nil).WithContext(ctx)
	req.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(roleSubscriber, []string{"random", "*"}))

	w := &responseTester{
		expectedStatusCode: http.StatusOK,
		expectedBody:       ":\nid: a\ndata: Foo\n\nevent: test\nid: b\ndata: Hello World\n\n",
		tb:                 t,
		cancel:             cancel,
	}

	hub.SubscribeHandler(w, req)
}

func TestSendMissedEvents(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		transport := createBoltTransport(t, 0, 0)
		ctx := t.Context()

		hub := createAnonymousDummy(t, WithLogger(transport.logger), WithTransport(transport), WithProtocolVersionCompatibility(7))

		require.NoError(t, transport.Dispatch(ctx, &Update{
			Topics: []string{"https://example.com/foos/a"},
			Event: Event{
				ID:   "a",
				Data: "d1",
			},
		}))
		require.NoError(t, transport.Dispatch(ctx, &Update{
			Topics: []string{"https://example.com/foos/b"},
			Event: Event{
				ID:   "b",
				Data: "d2",
			},
		}))

		// Using deprecated 'Last-Event-ID' query parameter
		go func() {
			ctx, cancel := context.WithCancel(t.Context())
			req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match_urlpattern=https://example.com/foos/:id&Last-Event-ID=a", nil).WithContext(ctx)

			w := &responseTester{
				expectedStatusCode: http.StatusOK,
				expectedBody:       ":\nid: b\ndata: d2\n\n",
				tb:                 t,
				cancel:             cancel,
			}

			hub.SubscribeHandler(w, req)
		}()

		go func() {
			ctx, cancel := context.WithCancel(t.Context())
			req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match_urlpattern=https://example.com/foos/:id&last_event_id=a", nil).WithContext(ctx)

			w := &responseTester{
				expectedStatusCode: http.StatusOK,
				expectedBody:       ":\nid: b\ndata: d2\n\n",
				tb:                 t,
				cancel:             cancel,
			}

			hub.SubscribeHandler(w, req)
		}()

		go func() {
			ctx, cancel := context.WithCancel(t.Context())
			req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match_urlpattern=https://example.com/foos/:id", nil).WithContext(ctx)
			req.Header.Add("Last-Event-ID", "a")

			w := &responseTester{
				expectedStatusCode: http.StatusOK,
				expectedBody:       ":\nid: b\ndata: d2\n\n",
				tb:                 t,
				cancel:             cancel,
			}

			hub.SubscribeHandler(w, req)
		}()

		synctest.Wait()
	})
}

func TestSendAllEvents(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		transport := createBoltTransport(t, 0, 0)
		hub := createAnonymousDummy(t, WithTransport(transport))
		ctx := t.Context()

		require.NoError(t, transport.Dispatch(ctx, &Update{
			Topics: []string{"https://example.com/foos/a"},
			Event: Event{
				ID:   "a",
				Data: "d1",
			},
		}))
		require.NoError(t, transport.Dispatch(ctx, &Update{
			Topics: []string{"https://example.com/foos/b"},
			Event: Event{
				ID:   "b",
				Data: "d2",
			},
		}))

		go func() {
			ctx, cancel := context.WithCancel(t.Context())
			req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match_urlpattern=https://example.com/foos/:id&last_event_id="+EarliestLastEventID, nil).WithContext(ctx)

			w := &responseTester{
				header:             http.Header{},
				expectedStatusCode: http.StatusOK,
				expectedBody:       ":\nid: a\ndata: d1\n\nid: b\ndata: d2\n\n",
				tb:                 t,
				cancel:             cancel,
			}

			hub.SubscribeHandler(w, req)
		}()

		go func() {
			ctx, cancel := context.WithCancel(t.Context())
			req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match_urlpattern=https://example.com/foos/:id", nil).WithContext(ctx)
			req.Header.Add("Last-Event-ID", EarliestLastEventID)

			w := &responseTester{
				header:             http.Header{},
				expectedStatusCode: http.StatusOK,
				expectedBody:       ":\nid: a\ndata: d1\n\nid: b\ndata: d2\n\n",
				tb:                 t,
				cancel:             cancel,
			}

			hub.SubscribeHandler(w, req)
		}()

		synctest.Wait()
	})
}

func TestUnknownLastEventID(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		transport := createBoltTransport(t, 0, 0)
		hub := createAnonymousDummy(t, WithLogger(transport.logger), WithTransport(transport))

		require.NoError(t, transport.Dispatch(t.Context(), &Update{
			Topics: []string{"https://example.com/foos/a"},
			Event: Event{
				ID:   "a",
				Data: "d1",
			},
		}))

		ctx := t.Context()

		go func(ctx context.Context) {
			c, cancel := context.WithCancel(ctx)
			req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match_urlpattern=https://example.com/foos/:id&last_event_id=unknown", nil).WithContext(c)

			w := &responseTester{
				header:             http.Header{},
				expectedStatusCode: http.StatusOK,
				expectedBody:       ":\nid: b\ndata: d2\n\n",
				tb:                 t,
				cancel:             cancel,
			}

			hub.SubscribeHandler(w, req)
			// "unknown" is not in the history, so nothing was replayed and the
			// cursor is the reserved "earliest", not the newest id in history.
			assert.Equal(t, EarliestLastEventID, w.Header().Get("Mercure-Last-Event-ID"))
		}(ctx)

		go func(ctx context.Context) {
			c, cancel := context.WithCancel(ctx)
			req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match_urlpattern=https://example.com/foos/:id", nil).WithContext(c)
			req.Header.Add("Last-Event-ID", "unknown")

			w := &responseTester{
				header:             http.Header{},
				expectedStatusCode: http.StatusOK,
				expectedBody:       ":\nid: b\ndata: d2\n\n",
				tb:                 t,
				cancel:             cancel,
			}

			hub.SubscribeHandler(w, req)
			// "unknown" is not in the history, so nothing was replayed and the
			// cursor is the reserved "earliest", not the newest id in history.
			assert.Equal(t, EarliestLastEventID, w.Header().Get("Mercure-Last-Event-ID"))
		}(ctx)

		for {
			transport.RLock()
			done := transport.subscribers.Len() == 2
			transport.RUnlock()

			if done {
				break
			}
		}

		require.NoError(t, transport.Dispatch(ctx, &Update{
			Topics: []string{"https://example.com/foos/b"},
			Event: Event{
				ID:   "b",
				Data: "d2",
			},
		}))

		synctest.Wait()
	})
}

func TestUnknownLastEventIDDoesNotLeakPrivateEventID(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		transport := createBoltTransport(t, 0, 0)
		hub := createAnonymousDummy(t, WithLogger(transport.logger), WithTransport(transport))

		// Public event the anonymous subscriber is authorized to read.
		require.NoError(t, transport.Dispatch(t.Context(), &Update{
			Topics: []string{"https://example.com/foos/a"},
			Event:  Event{ID: "a", Data: "d1"},
		}))
		// Private event the anonymous subscriber is NOT authorized to
		// read. Its id must not appear in the Last-Event-ID response.
		require.NoError(t, transport.Dispatch(t.Context(), &Update{
			Topics:  []string{"https://example.com/foos/b"},
			Private: true,
			Event:   Event{ID: "b", Data: "secret"},
		}))

		ctx := t.Context()

		go func(ctx context.Context) {
			c, cancel := context.WithCancel(ctx)
			req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match_urlpattern=https://example.com/foos/:id&last_event_id=unknown", nil).WithContext(c)

			w := &responseTester{
				header:             http.Header{},
				expectedStatusCode: http.StatusOK,
				expectedBody:       ":\nid: c\ndata: d3\n\n",
				tb:                 t,
				cancel:             cancel,
			}

			hub.SubscribeHandler(w, req)

			cursor := w.Header().Get("Mercure-Last-Event-ID")
			// The private "b" must not leak, even though it is the most
			// recent in-history event. Nothing was replayed either, since
			// "unknown" is not in the history, so the cursor is the reserved
			// "earliest" rather than the authorized "a" — which the
			// subscriber never received.
			assert.NotEqual(t, "b", cursor)
			assert.Equal(t, EarliestLastEventID, cursor)
		}(ctx)

		for {
			transport.RLock()
			done := transport.subscribers.Len() == 1
			transport.RUnlock()

			if done {
				break
			}
		}

		require.NoError(t, transport.Dispatch(ctx, &Update{
			Topics: []string{"https://example.com/foos/c"},
			Event:  Event{ID: "c", Data: "d3"},
		}))

		synctest.Wait()
	})
}

func TestUnknownLastEventIDEmptyHistory(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		transport := createBoltTransport(t, 0, 0)
		hub := createAnonymousDummy(t, WithTransport(transport))

		ctx := t.Context()

		go func() {
			ctx, cancel := context.WithCancel(ctx)
			req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match_urlpattern=https://example.com/foos/:id&last_event_id=unknown", nil).WithContext(ctx)

			w := &responseTester{
				header:             http.Header{},
				expectedStatusCode: http.StatusOK,
				expectedBody:       ":\nid: b\ndata: d2\n\n",
				tb:                 t,
				cancel:             cancel,
			}

			hub.SubscribeHandler(w, req)
			assert.Equal(t, EarliestLastEventID, w.Header().Get("Mercure-Last-Event-ID"))
		}()

		go func() {
			ctx, cancel := context.WithCancel(ctx)
			req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match_urlpattern=https://example.com/foos/:id", nil).WithContext(ctx)
			req.Header.Add("Last-Event-ID", "unknown")

			w := &responseTester{
				header:             http.Header{},
				expectedStatusCode: http.StatusOK,
				expectedBody:       ":\nid: b\ndata: d2\n\n",
				tb:                 t,
				cancel:             cancel,
			}

			hub.SubscribeHandler(w, req)
			assert.Equal(t, EarliestLastEventID, w.Header().Get("Mercure-Last-Event-ID"))
		}()

		for {
			transport.RLock()
			done := transport.subscribers.Len() == 2
			transport.RUnlock()

			if done {
				break
			}
		}

		require.NoError(t, transport.Dispatch(ctx, &Update{
			Topics: []string{"https://example.com/foos/b"},
			Event: Event{
				ID:   "b",
				Data: "d2",
			},
		}))

		synctest.Wait()
	})
}

// A present-but-empty last_event_id still gets a Mercure-Last-Event-ID
// response field, as the protocol requires whenever the parameter is present.
func TestEmptyLastEventIDGetsResponseHeader(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		hub := createAnonymousDummy(t)
		transport, _ := hub.transport.(*LocalTransport)

		ctx := t.Context()

		go func() {
			ctx, cancel := context.WithCancel(ctx)
			req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match=https://example.com/foo&last_event_id=", nil).WithContext(ctx)

			w := &responseTester{
				header:             http.Header{},
				expectedStatusCode: http.StatusOK,
				expectedBody:       ":\nid: e1\ndata: d\n\n",
				tb:                 t,
				cancel:             cancel,
			}

			hub.SubscribeHandler(w, req)
			assert.Equal(t, EarliestLastEventID, w.Header().Get("Mercure-Last-Event-ID"))
		}()

		for {
			transport.RLock()
			done := transport.subscribers.Len() == 1
			transport.RUnlock()

			if done {
				break
			}
		}

		require.NoError(t, transport.Dispatch(ctx, &Update{
			Topics: []string{"https://example.com/foo"},
			Event:  Event{ID: "e1", Data: "d"},
		}))

		synctest.Wait()
	})
}

func TestSubscribeHeartbeat(t *testing.T) {
	hub := createAnonymousDummy(t, WithHeartbeat(5*time.Millisecond))
	s, _ := hub.transport.(*LocalTransport)
	ctx := t.Context()

	go func() {
		for {
			s.RLock()
			empty := s.subscribers.Len() == 0
			s.RUnlock()

			if empty {
				continue
			}

			_ = hub.transport.Dispatch(ctx, &Update{
				Topics: []string{"https://example.com/books/1"},
				Event:  Event{Data: "Hello World", ID: "b"},
			})

			return
		}
	}()

	ctx, cancel := context.WithCancel(ctx)
	req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match=https://example.com/books/1&match_urlpattern=https://example.com/reviews/:id", nil).WithContext(ctx)

	w := &responseTester{
		expectedStatusCode: http.StatusOK,
		expectedBody:       ":\nid: b\ndata: Hello World\n\n:\n",
		tb:                 t,
		cancel:             cancel,
	}

	hub.SubscribeHandler(w, req)
}

func TestSubscribeExpires(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t, WithWriteTimeout(0), WithDispatchTimeout(0), WithHeartbeat(500*time.Millisecond))
	token := jwt.New(jwt.SigningMethodHS256)
	token.Header["typ"] = atJWTType
	token.Claims = &claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    testIssuer,
			Audience:  jwt.ClaimStrings{testResourceIdentifier},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Second)),
		},
		AuthorizationDetails: subscribeDetailsFromMatchers(nil, TopicMatcher{Type: MatcherTypeExact, Pattern: "*"}),
	}

	signedString, err := token.SignedString([]byte("subscriber"))
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match=foo", nil)
	req.Header.Add("Authorization", bearerPrefix+signedString)

	w := newSubscribeRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() {
		require.NoError(t, resp.Body.Close())
	})

	assert.Equal(t, 200, resp.StatusCode)
}

func BenchmarkSubscribe(b *testing.B) {
	for b.Loop() {
		subscribe(b, 1000)
	}
}

// hubShutdownTestHub builds a hub with a caller-controlled context so tests
// can cancel the hub independently of the subscriber's request context.
func hubShutdownTestHub(ctx context.Context, tb testing.TB, writeTimeout time.Duration) *Hub {
	tb.Helper()

	tms, err := NewTopicMatcherStore(0)
	require.NoError(tb, err)

	h, err := NewHub(ctx,
		WithAnonymous(),
		WithIssuers([]Issuer{{
			Identifier: testIssuer,
			Publisher:  Static{Key: []byte("publisher"), Algorithm: jwt.SigningMethodHS256.Name},
			Subscriber: Static{Key: []byte("subscriber"), Algorithm: jwt.SigningMethodHS256.Name},
		}}),
		WithResourceIdentifier(testResourceIdentifier),
		WithTopicMatcherStore(tms),
		WithWriteTimeout(writeTimeout),
	)
	require.NoError(tb, err)

	return h
}

// TestShutdownKeepsSubscribersWhenWriteTimeoutEnabled verifies the graceful
// drain contract: when the hub context is cancelled (Caddy stopping, pod
// SIGTERM, ...) and writeTimeout is set, subscribers stay connected until
// their per-connection disconnection timer fires or the client disconnects.
func TestShutdownKeepsSubscribersWhenWriteTimeoutEnabled(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		hubCtx, cancelHub := context.WithCancel(t.Context())
		hub := hubShutdownTestHub(hubCtx, t, 5*time.Minute)
		transport, _ := hub.transport.(*LocalTransport)

		go func() {
			req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match=https://example.com/books/1", nil).WithContext(t.Context())
			hub.SubscribeHandler(newSubscribeRecorder(), req)
		}()

		waitSubscribers(t, transport, 1)

		// Simulate hub shutdown.
		cancelHub()
		synctest.Wait()

		transport.RLock()
		n := transport.subscribers.Len()
		transport.RUnlock()
		assert.Equal(t, 1, n, "subscriber must stay connected when writeTimeout is set; disconnect timer is the drain mechanism")
	})
}

// TestShutdownClosesSubscribersWhenWriteTimeoutDisabled covers the escape
// hatch: with writeTimeout == 0 there is no per-connection disconnect timer,
// so the hub context cancel must still terminate subscribers — otherwise
// http.Server.Shutdown would hang forever on active handlers.
func TestShutdownClosesSubscribersWhenWriteTimeoutDisabled(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		hubCtx, cancelHub := context.WithCancel(t.Context())
		hub := hubShutdownTestHub(hubCtx, t, 0)
		transport, _ := hub.transport.(*LocalTransport)

		go func() {
			req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match=https://example.com/books/1", nil).WithContext(t.Context())
			hub.SubscribeHandler(newSubscribeRecorder(), req)
		}()

		waitSubscribers(t, transport, 1)

		cancelHub()
		synctest.Wait()

		transport.RLock()
		n := transport.subscribers.Len()
		transport.RUnlock()
		assert.Equal(t, 0, n, "subscriber must exit on hub shutdown when writeTimeout is 0")
	})
}

// The disconnection timer is armed with time.Until(disconnectionTime), so a
// disconnectionTime in the past closes the connection as soon as it opens.
// Reachable whenever the write deadline is nearer than dispatchTimeout.
func TestNewResponseControllerDisconnectionTimeStaysInTheFuture(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name            string
		writeTimeout    time.Duration
		dispatchTimeout time.Duration
		tokenExpiresIn  time.Duration
	}{
		{name: "token expiring sooner than dispatchTimeout", writeTimeout: 600 * time.Second, dispatchTimeout: 5 * time.Second, tokenExpiresIn: 2 * time.Second},
		{name: "token expiring at exactly dispatchTimeout", writeTimeout: 600 * time.Second, dispatchTimeout: 5 * time.Second, tokenExpiresIn: 5 * time.Second},
		{name: "dispatchTimeout larger than writeTimeout", writeTimeout: 5 * time.Second, dispatchTimeout: 10 * time.Second},
		{name: "healthy defaults", writeTimeout: DefaultWriteTimeout, dispatchTimeout: DefaultDispatchTimeout},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := &Hub{opt: &opt{writeTimeout: tc.writeTimeout, dispatchTimeout: tc.dispatchTimeout}}

			s := &LocalSubscriber{}
			if tc.tokenExpiresIn != 0 {
				s.Claims = &claims{RegisteredClaims: jwt.RegisteredClaims{
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(tc.tokenExpiresIn)),
				}}
			}

			rc := h.newResponseController(httptest.NewRecorder(), s, 0)

			assert.True(t, rc.disconnectionTime.After(time.Now()),
				"disconnectionTime is %v in the past, the connection would close immediately", time.Until(rc.disconnectionTime))
			assert.False(t, rc.disconnectionTime.After(rc.writeDeadline),
				"disconnectionTime must not outlive the write deadline")
		})
	}
}

// With neither a write timeout nor a token expiry there is no deadline, so no
// disconnection timer is armed and the zero time must be preserved.
func TestNewResponseControllerNoDeadline(t *testing.T) {
	t.Parallel()

	h := &Hub{opt: &opt{writeTimeout: 0, dispatchTimeout: DefaultDispatchTimeout}}
	rc := h.newResponseController(httptest.NewRecorder(), &LocalSubscriber{}, 0)

	assert.True(t, rc.writeDeadline.IsZero())
	assert.True(t, rc.disconnectionTime.IsZero())
}

// refusingTransport records dispatched updates and refuses to register
// subscribers, to exercise the registration-failure path.
type refusingTransport struct {
	dispatched []*Update
}

func (t *refusingTransport) Dispatch(_ context.Context, u *Update) error {
	t.dispatched = append(t.dispatched, u)

	return nil
}

func (t *refusingTransport) AddSubscriber(context.Context, *LocalSubscriber) error {
	return ErrClosedTransport
}

func (t *refusingTransport) RemoveSubscriber(context.Context, *LocalSubscriber) error { return nil }

func (t *refusingTransport) Close(context.Context) error { return nil }

// A registration that fails must not announce a subscription that never
// existed, nor take it back with a compensating active:false.
func TestNoSubscriptionEventWhenRegistrationFails(t *testing.T) {
	t.Parallel()

	transport := &refusingTransport{}
	hub := createAnonymousDummy(t, WithSubscriptions(), WithTransport(transport))

	req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match=https://example.com/foo", nil)
	w := httptest.NewRecorder()

	hub.SubscribeHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() {
		require.NoError(t, resp.Body.Close())
	})

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	assert.Empty(t, transport.dispatched)
}

// The subscription is announced once it exists, so a subscriber authorized for
// the subscriptions namespace sees its own arrival and needs no reconciliation
// against the snapshot it fetched from the subscription API.
func TestSubscriptionEventReachesTheSubscriberItDescribes(t *testing.T) {
	t.Parallel()

	hub := createDummy(t, WithSubscriptions())

	req := httptest.NewRequest(http.MethodGet,
		defaultHubURL+"?match_urlpattern=/.well-known/mercure/subscriptions/:mt/:m/:s", nil)
	req.AddCookie(&http.Cookie{
		Name:  defaultCookieName,
		Value: createDummyAuthorizedJWT(roleSubscriber, []string{"*"}),
	})

	w := newSubscribeRecorder()

	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()

	hub.SubscribeHandler(w, req.WithContext(ctx))

	body := w.Body.String()
	assert.Contains(t, body, "event: mercure")
	assert.Contains(t, body, `"active": true`)
	assert.Contains(t, body, `"match": "/.well-known/mercure/subscriptions/:mt/:m/:s"`)
}

// A subscription response must state the media type it is framed in. Every
// other assertion about a subscription looks at the body, so nothing else
// would notice the hub answering with the wrong Content-Type.
func TestSubscribeContentType(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t)

	ctx, cancel := context.WithCancel(t.Context())
	req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match=https://example.com/foo", nil).WithContext(ctx)

	w := &responseTester{
		header:             http.Header{},
		expectedStatusCode: http.StatusOK,
		expectedBody:       ":\n",
		tb:                 t,
		cancel:             cancel,
	}

	hub.SubscribeHandler(w, req)

	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
}

// An Events Query is answered in the media type it negotiated, which is the
// only framing this hub has to offer so far.
func TestEventsQuerySubscribeContentType(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t, WithEventsQuery())

	ctx, cancel := context.WithCancel(t.Context())
	req := eventsQueryRequest(ctx, `{"url": ["https://example.com/books/1"], "events": {}}`)

	// The delimiter that opens the stream is enough to know it started.
	w := newEventsQueryTester(1, cancel)

	hub.SubscribeHandler(w, req)

	mediaType, params, err := mime.ParseMediaType(w.Header().Get("Content-Type"))
	require.NoError(t, err)
	assert.Equal(t, multipartDigestContentType, mediaType)
	assert.NotEmpty(t, params["boundary"], "every connection is framed by its own boundary")
}

// A subscription is answered with the media types a QUERY body can express it
// in, so a client learns what the hub reads (RFC 10008, Section 3).
func TestSubscribeAcceptQuery(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t)

	ctx, cancel := context.WithCancel(t.Context())
	req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match=https://example.com/foo", nil).WithContext(ctx)

	w := &responseTester{
		header:             http.Header{},
		expectedStatusCode: http.StatusOK,
		expectedBody:       ":\n",
		tb:                 t,
		cancel:             cancel,
	}

	hub.SubscribeHandler(w, req)

	assert.Equal(t, urlEncodedMediaType, w.Header().Get("Accept-Query"))
}

// A hub serving Events Query advertises every media type its parsers read, so
// this expectation grows as parsers are added.
func TestEventsQuerySubscribeAcceptQuery(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t, WithEventsQuery())

	ctx, cancel := context.WithCancel(t.Context())
	req := eventsQueryRequest(ctx, `{"url": ["https://example.com/foo"], "events": {}}`)

	// The delimiter that opens the stream is enough to know it started.
	w := newEventsQueryTester(1, cancel)

	hub.SubscribeHandler(w, req)

	assert.Equal(t, "application/events+json, application/x-www-form-urlencoded",
		w.Header().Get("Accept-Query"))
}

// A subscription asks intermediaries to forward each chunk as it arrives,
// which a buffered response would otherwise defeat.
func TestSubscribeIncremental(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t)

	ctx, cancel := context.WithCancel(t.Context())
	req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match=https://example.com/foo", nil).WithContext(ctx)

	w := &responseTester{
		header:             http.Header{},
		expectedStatusCode: http.StatusOK,
		expectedBody:       ":\n",
		tb:                 t,
		cancel:             cancel,
	}

	hub.SubscribeHandler(w, req)

	assert.Equal(t, "?1", w.Header().Get("Incremental"))
}

// A subscription expressed as an Events Query receives the updates it asked
// for. Nothing negotiates a carrier yet, so it is answered as an event
// stream, like any other subscription.
func TestEventsQuerySubscribe(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t, WithEventsQuery())
	ctx := t.Context()

	go func() {
		s := hub.transport.(*LocalTransport)

		var ready bool

		for !ready {
			s.RLock()
			ready = s.subscribers.Len() == 1
			s.RUnlock()
		}

		_ = hub.transport.Dispatch(ctx, &Update{
			Topics: []string{"https://example.com/books/1"},
			Event:  Event{Data: "Hello World", ID: "b"},
		})
	}()

	reqCtx, cancel := context.WithCancel(t.Context())
	req := eventsQueryRequest(reqCtx, `{"url": ["https://example.com/books/1"], "events": {}}`)

	// One delimiter opens the stream, the next closes the notification.
	w := newEventsQueryTester(2, cancel)
	hub.SubscribeHandler(w, req)

	assertNotification(t, w, "b", "Hello World")
}

// The parameters reach the same matcher implementation the query component
// does, so a pattern matcher works identically in a request body.
func TestEventsQuerySubscribeURLPattern(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t, WithEventsQuery())
	ctx := t.Context()

	go func() {
		s := hub.transport.(*LocalTransport)

		var ready bool

		for !ready {
			s.RLock()
			ready = s.subscribers.Len() == 1
			s.RUnlock()
		}

		_ = hub.transport.Dispatch(ctx, &Update{
			Topics: []string{"https://example.com/books/1"},
			Event:  Event{Data: "Hello World", ID: "b"},
		})
	}()

	reqCtx, cancel := context.WithCancel(t.Context())
	req := eventsQueryRequest(reqCtx,
		`{"url": {"match_urlpattern": ["https://example.com/books/:id"]}, "events": {}}`)

	// One delimiter opens the stream, the next closes the notification.
	w := newEventsQueryTester(2, cancel)
	hub.SubscribeHandler(w, req)

	assertNotification(t, w, "b", "Hello World")
}

// A subscription naming no topic is refused the same way a subscription URL
// without a match parameter is.
func TestEventsQuerySubscribeWithoutTopics(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t, WithEventsQuery())

	w := newSubscribeRecorder()
	hub.SubscribeHandler(w, eventsQueryRequest(t.Context(), `{"events": {}}`))

	assert.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
}

// eventsQueryRequest builds a subscription request carrying the given body,
// expressed in the media type an Events Query is canonically written in.
func eventsQueryRequest(ctx context.Context, body string) *http.Request {
	req := httptest.NewRequest(methodQuery, defaultHubURL, strings.NewReader(body)).WithContext(ctx)
	req.Header.Set("Content-Type", eventsJSONMediaType)

	return req
}

// A subscription resuming from a cursor in its parameters is answered with
// the Mercure-Last-Event-Id field the protocol requires.
func TestEventsQuerySubscribeLastEventID(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t, WithEventsQuery())

	reqCtx, cancel := context.WithCancel(t.Context())
	defer cancel()

	req := eventsQueryRequest(reqCtx,
		`{"url": ["https://example.com/books/1"], "events": {}, "last_event_id": "urn:uuid:0198c1f2-3f4a-7000-8000-9abcdef01234"}`)

	// The delimiter that opens the stream is enough to know it started.
	w := newEventsQueryTester(1, cancel)
	hub.SubscribeHandler(w, req)

	assert.Equal(t, EarliestLastEventID, w.Header().Get("Mercure-Last-Event-ID"))
}

// A malformed body is a bad request: the type is declared and understood, but
// the content does not match it (RFC 10008, Section 2.3).
func TestEventsQuerySubscribeRejectsMalformedBody(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t, WithEventsQuery())

	w := httptest.NewRecorder()
	hub.SubscribeHandler(w, eventsQueryRequest(t.Context(), `{"url": 42, "events": {}}`))

	resp := w.Result()

	t.Cleanup(func() { assert.NoError(t, resp.Body.Close()) })

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// A subscription is served for as long as it asked. The dispatch of margin
// belongs to the hub's own deadline, so it is not taken out of the bound the
// client chose, whichever side of a dispatch that bound falls on.
func TestNewResponseControllerServesTheRequestedDuration(t *testing.T) {
	t.Parallel()

	for _, requested := range []time.Duration{
		30 * time.Second,
		6 * time.Second,
		5 * time.Second,
		4 * time.Second,
		time.Second,
	} {
		t.Run(requested.String(), func(t *testing.T) {
			t.Parallel()

			h := &Hub{opt: &opt{writeTimeout: 10 * time.Minute, dispatchTimeout: 5 * time.Second}}

			rc := h.newResponseController(httptest.NewRecorder(), &LocalSubscriber{}, requested)

			assert.InDelta(t, requested, time.Until(rc.disconnectionTime), float64(500*time.Millisecond))
			assert.True(t, rc.writeDeadline.After(rc.disconnectionTime),
				"the connection must outlive the notifications it carries")
		})
	}
}

// A hub with no deadline of its own stops when the subscription asked it to.
func TestNewResponseControllerRequestedDurationWithoutHubDeadline(t *testing.T) {
	t.Parallel()

	h := &Hub{opt: &opt{writeTimeout: 0, dispatchTimeout: 5 * time.Second}}

	rc := h.newResponseController(httptest.NewRecorder(), &LocalSubscriber{}, 30*time.Second)

	assert.InDelta(t, 30*time.Second, time.Until(rc.disconnectionTime), float64(500*time.Millisecond))
	// The connection keeps the deadline it had, which is none: what was asked
	// for bounds the notifications, not the socket.
	assert.True(t, rc.writeDeadline.IsZero())
}

// A bound looser than the hub's own changes nothing: a subscription may only
// shorten the period it is served for.
func TestNewResponseControllerRequestedDurationOnlyShortens(t *testing.T) {
	t.Parallel()

	h := &Hub{opt: &opt{writeTimeout: time.Minute, dispatchTimeout: 5 * time.Second}}

	rc := h.newResponseController(httptest.NewRecorder(), &LocalSubscriber{}, time.Hour)

	assert.False(t, rc.disconnectionTime.After(rc.writeDeadline.Add(-5*time.Second)),
		"a longer request must not extend the period served")
}

// A token expiring inside the dispatch margin leaves no margin to give away,
// and a request cannot buy more time than the token grants.
func TestNewResponseControllerRequestedDurationCannotOutlastTheToken(t *testing.T) {
	t.Parallel()

	h := &Hub{opt: &opt{writeTimeout: 0, dispatchTimeout: 5 * time.Second}}

	s := &LocalSubscriber{Subscriber: Subscriber{}}
	s.Claims = &claims{RegisteredClaims: jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(2 * time.Second)),
	}}

	rc := h.newResponseController(httptest.NewRecorder(), s, 30*time.Second)

	assert.Equal(t, rc.writeDeadline, rc.disconnectionTime,
		"with the margin already spent the hub stops at the deadline itself")
	assert.False(t, time.Until(rc.disconnectionTime) > 2*time.Second)
}

// The Events response field states the period notifications will be sent for,
// in whole seconds rounded up: the hub may stop before the bound it
// advertised, but must never still be sending after it.
func TestSendHeadersEventsDuration(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		requested time.Duration
		want      string
	}{
		{"whole seconds", 30 * time.Second, "duration=30"},
		{"rounded up", 1500 * time.Millisecond, "duration=2"},
		{"sub-second rounds up to a second", 500 * time.Millisecond, "duration=1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			hub := createAnonymousDummy(t, WithEventsQuery(),
				WithWriteTimeout(10*time.Minute), WithDispatchTimeout(5*time.Second))

			s := &LocalSubscriber{}
			w := httptest.NewRecorder()

			rc := hub.newResponseController(w, s, tc.requested)
			hub.sendHeaders(t.Context(), w, s, []string{eventStreamContentType}, ":\n", rc.disconnectionTime)

			assert.Equal(t, tc.want, w.Header().Get("Events"))
		})
	}
}

// A hub not serving Events Query sends none of its fields.
func TestSendHeadersOmitsEventsDurationWhenDisabled(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t, WithWriteTimeout(10*time.Minute))

	s := &LocalSubscriber{}
	w := httptest.NewRecorder()

	rc := hub.newResponseController(w, s, 0)
	hub.sendHeaders(t.Context(), w, s, []string{eventStreamContentType}, ":\n", rc.disconnectionTime)

	assert.Empty(t, w.Header().Values("Events"))
}

// A hub that will not stop has no duration to state, and 0 would read as "no
// notifications will be served".
func TestSendHeadersOmitsEventsDurationWithoutDeadline(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t, WithEventsQuery(), WithWriteTimeout(0), WithDispatchTimeout(0))

	s := &LocalSubscriber{}
	w := httptest.NewRecorder()

	rc := hub.newResponseController(w, s, 0)
	hub.sendHeaders(t.Context(), w, s, []string{eventStreamContentType}, ":\n", rc.disconnectionTime)

	assert.Empty(t, w.Header().Values("Events"))
}

// A hub with no deadline of its own still stops when the subscription said
// to: the bound it advertises is one it enforces.
func TestEventsQuerySubscribeStopsAfterTheRequestedDuration(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t, WithEventsQuery(), WithWriteTimeout(0), WithDispatchTimeout(0))

	req := eventsQueryRequest(t.Context(), `{"url": ["https://example.com/books/1"], "events": {}}`)
	req.Header.Set("Events", "duration=0.2")

	w := writeDeadlineRecorder{httptest.NewRecorder()}

	served := make(chan struct{})

	go func() {
		defer close(served)

		hub.SubscribeHandler(w, req)
	}()

	select {
	case <-served:
	case <-time.After(5 * time.Second):
		t.Fatal("the hub streamed past the duration the subscription asked for")
	}

	assert.Equal(t, http.StatusOK, w.Code)
	// The stream carried no notification and the hub closed it itself, so it
	// is a preamble followed by the close-delimiter.
	assert.True(t, strings.HasSuffix(w.Body.String(), "--\r\n"),
		"the stream must end with the close-delimiter, got %q", w.Body.String())
	assert.Equal(t, "duration=1", w.Header().Get("Events"))
}

// writeDeadlineRecorder accepts the write deadlines a subscription sets on
// its connection, which httptest.ResponseRecorder does not support at all.
type writeDeadlineRecorder struct {
	*httptest.ResponseRecorder
}

func (writeDeadlineRecorder) SetWriteDeadline(time.Time) error { return nil }

// assertNotification reads back the one notification a stream carries, by the
// length its message declares rather than the delimiter.
func assertNotification(t *testing.T, w *eventsQueryTester, id, data string) {
	t.Helper()

	_, params, err := mime.ParseMediaType(w.Header().Get("Content-Type"))
	require.NoError(t, err)
	require.NotEmpty(t, params["boundary"])

	// Close the body so the parser sees a complete multipart message: the hub
	// writes the trailer only when it ends the response itself.
	r := multipart.NewReader(strings.NewReader(w.String()+"--\r\n"), params["boundary"])

	part, err := r.NextPart()
	require.NoError(t, err)
	assert.Empty(t, part.Header.Get("Content-Type"), "a notification takes the carrier's default")

	header, body := readNotification(t, bufio.NewReader(part))
	assert.Equal(t, data, body)
	assert.Equal(t, id, header.Get("Event-ID"))

	// Topics never reach a subscriber, over either carrier.
	assert.Empty(t, header.Get("Topic"))
}

// eventsQueryTester collects a notifications stream, ending the request once
// it has seen the delimiter that many times. Unlike responseTester it cannot
// match against an expected body: the boundary is random per connection, so
// the response is only knowable once parsed.
type eventsQueryTester struct {
	mu       sync.Mutex
	body     strings.Builder
	boundary string
	// wanted is the number of delimiters to wait for before cancelling.
	wanted int
	cancel context.CancelFunc
	header http.Header
}

func newEventsQueryTester(wanted int, cancel context.CancelFunc) *eventsQueryTester {
	return &eventsQueryTester{wanted: wanted, cancel: cancel, header: http.Header{}}
}

func (t *eventsQueryTester) Header() http.Header { return t.header }

func (t *eventsQueryTester) WriteHeader(int) {}

func (t *eventsQueryTester) Flush() {}

func (t *eventsQueryTester) SetWriteDeadline(_ time.Time) error { return nil }

func (t *eventsQueryTester) Write(buf []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.body.Write(buf)

	if t.boundary == "" {
		if _, params, err := mime.ParseMediaType(t.header.Get("Content-Type")); err == nil {
			t.boundary = params["boundary"]
		}
	}

	if t.boundary != "" && strings.Count(t.body.String(), "--"+t.boundary) >= t.wanted {
		t.cancel()
	}

	return len(buf), nil
}

func (t *eventsQueryTester) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.body.String()
}
