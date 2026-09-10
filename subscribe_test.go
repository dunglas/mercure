package mercure

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"slices"
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

// sseSubscriptions decodes every subscription document carried by an SSE
// stream. Assertions then run against the document rather than against its
// serialised form, so they survive a change of JSON formatting.
func sseSubscriptions(tb testing.TB, stream string) []subscription {
	tb.Helper()

	var subs []subscription

	for frame := range strings.SplitSeq(stream, "\n\n") {
		var data []string

		for line := range strings.SplitSeq(frame, "\n") {
			if after, ok := strings.CutPrefix(line, "data:"); ok {
				data = append(data, strings.TrimPrefix(after, " "))
			}
		}

		if len(data) == 0 {
			continue
		}

		// Per the SSE grammar the data lines of a frame are joined with LF.
		var sub subscription
		require.NoError(tb, json.Unmarshal([]byte(strings.Join(data, "\n")), &sub))

		subs = append(subs, sub)
	}

	return subs
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
			Data:   "Hello World",
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
			Data:   "Hello World", ID: "b",
		})
	}()

	reqCtx, cancel := context.WithCancel(t.Context())
	// Topics travel in the QUERY request body instead of the URL.
	body := url.Values{"match": {"https://example.com/books/1"}}.Encode()
	req := httptest.NewRequest(methodQuery, defaultHubURL, strings.NewReader(body)).WithContext(reqCtx)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := &responseTester{
		expectedStatusCode: http.StatusOK,
		expectedBody:       ":\ntopics: https://example.com/books/1\nid: b\ndata: Hello World\n\n",
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
			Data:   "Hello World", ID: "a",
		})
		_ = hub.transport.Dispatch(ctx, &Update{
			Topics: []string{"https://example.com/books/1"},
			Data:   "Hello World", ID: "b",
		})
		_ = hub.transport.Dispatch(ctx, &Update{
			Topics: []string{"https://example.com/reviews/22"},
			Data:   "Great", ID: "c",
		})
		_ = hub.transport.Dispatch(ctx, &Update{
			Topics: []string{"https://example.com/hub?topic=faulty{iri"},
			Data:   "Faulty IRI", ID: "d",
		})
		_ = hub.transport.Dispatch(ctx, &Update{
			Topics: []string{"string"},
			Data:   "string", ID: "e",
		})
	}()

	var wg sync.WaitGroup

	for range numberOfSubscribers {
		wg.Go(func() {
			ctx, cancel := context.WithCancel(tb.Context())
			req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match=https://example.com/books/1&match=string&match_urlpattern=https://example.com/reviews/:id&match=https://example.com/hub?topic=faulty{iri", nil).WithContext(ctx)

			w := &responseTester{
				expectedStatusCode: http.StatusOK,
				expectedBody:       ":\ntopics: https://example.com/books/1\nid: b\ndata: Hello World\n\ntopics: https://example.com/reviews/22\nid: c\ndata: Great\n\ntopics: https://example.com/hub?topic=faulty{iri\nid: d\ndata: Faulty IRI\n\ntopics: string\nid: e\ndata: string\n\n",
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
				Topics: []string{"https://example.com/reviews/21"},
				Data:   "Foo", ID: "a",
				Private: true,
			})
			_ = hub.transport.Dispatch(ctx, &Update{
				Topics: []string{"https://example.com/reviews/22"},
				Data:   "Hello World", ID: "b", Type: "test",
				Private: true,
			})
			_ = hub.transport.Dispatch(ctx, &Update{
				Topics: []string{"https://example.com/reviews/23"},
				Data:   "Great", ID: "c", Retry: 1,
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
		expectedBody:       ":\nevent: test\ntopics: https://example.com/reviews/22\nid: b\ndata: Hello World\n\nretry: 1\ntopics: https://example.com/reviews/23\nid: c\ndata: Great\n\n",
		tb:                 t,
		cancel:             cancel,
	}

	hub.SubscribeHandler(w, req)
}

func TestSubscriptionEvents(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
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

			subs := sseSubscriptions(t, bodyContent)
			require.NotEmpty(t, subs)

			var announced, withdrawn []subscription

			for _, sub := range subs {
				if sub.Active {
					announced = append(announced, sub)
				} else {
					withdrawn = append(withdrawn, sub)
				}
			}

			assert.NotEmpty(t, announced, "no subscription was announced")
			assert.NotEmpty(t, withdrawn, "the disconnection was never announced")

			for _, sub := range subs {
				assert.Equal(t, "subscription", sub.Type)
				assert.Regexp(t, `^urn:uuid:`, sub.Subscriber)
			}

			i := slices.IndexFunc(subs, func(sub subscription) bool { return sub.Match == "https://example.com" })
			require.GreaterOrEqual(t, i, 0, "no event described the example.com subscription")

			assert.Equal(t, string(MatcherTypeExact), subs[i].MatchType)
			assert.Regexp(t, `^/\.well-known/mercure/subscriptions/exact/https%3A%2F%2Fexample\.com/`, subs[i].ID)
			assert.Equal(t, map[string]any{"foo": "bar"}, subs[i].Payload)
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
			// Both subscribers above are registered once they are durably
			// blocked waiting for updates.
			synctest.Wait()

			ctx, cancelRequest2 := context.WithCancel(t.Context())
			req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match=https://example.com", nil).WithContext(ctx)
			req.AddCookie(&http.Cookie{Name: defaultCookieName, Value: createDummyAuthorizedJWT(roleSubscriber, []string{"https://example.com"})})

			w := &responseTester{
				expectedStatusCode: http.StatusOK,
				expectedBody:       ":\n",
				tb:                 t,
				cancel:             cancelRequest2,
			}
			hub.SubscribeHandler(w, req)

			// This subscriber is gone; wait for the resulting "active": false
			// update to reach the subscriber above before tearing it down.
			synctest.Wait()

			cancel2()
			cancel1()
		})

		wg.Wait()
	})
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
				Topics: []string{"https://example.com/reviews/21"},
				Data:   "Foo", ID: "a",
				Private: true,
			})
			_ = hub.transport.Dispatch(ctx, &Update{
				Topics: []string{"https://example.com/reviews/22"},
				Data:   "Hello World", ID: "b", Type: "test",
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
		expectedBody:       ":\ntopics: https://example.com/reviews/21\nid: a\ndata: Foo\n\nevent: test\ntopics: https://example.com/reviews/22\nid: b\ndata: Hello World\n\n",
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
			ID:     "a",
			Data:   "d1",
		}))
		require.NoError(t, transport.Dispatch(ctx, &Update{
			Topics: []string{"https://example.com/foos/b"},
			ID:     "b",
			Data:   "d2",
		}))

		// Using deprecated 'Last-Event-ID' query parameter
		go func() {
			ctx, cancel := context.WithCancel(t.Context())
			req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match_urlpattern=https://example.com/foos/:id&Last-Event-ID=a", nil).WithContext(ctx)

			w := &responseTester{
				expectedStatusCode: http.StatusOK,
				expectedBody:       ":\ntopics: https://example.com/foos/b\nid: b\ndata: d2\n\n",
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
				expectedBody:       ":\ntopics: https://example.com/foos/b\nid: b\ndata: d2\n\n",
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
				expectedBody:       ":\ntopics: https://example.com/foos/b\nid: b\ndata: d2\n\n",
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
			ID:     "a",
			Data:   "d1",
		}))
		require.NoError(t, transport.Dispatch(ctx, &Update{
			Topics: []string{"https://example.com/foos/b"},
			ID:     "b",
			Data:   "d2",
		}))

		go func() {
			ctx, cancel := context.WithCancel(t.Context())
			req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match_urlpattern=https://example.com/foos/:id&last_event_id="+EarliestLastEventID, nil).WithContext(ctx)

			w := &responseTester{
				header:             http.Header{},
				expectedStatusCode: http.StatusOK,
				expectedBody:       ":\ntopics: https://example.com/foos/a\nid: a\ndata: d1\n\ntopics: https://example.com/foos/b\nid: b\ndata: d2\n\n",
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
				expectedBody:       ":\ntopics: https://example.com/foos/a\nid: a\ndata: d1\n\ntopics: https://example.com/foos/b\nid: b\ndata: d2\n\n",
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
			ID:     "a",
			Data:   "d1",
		}))

		ctx := t.Context()

		go func(ctx context.Context) {
			c, cancel := context.WithCancel(ctx)
			req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match_urlpattern=https://example.com/foos/:id&last_event_id=unknown", nil).WithContext(c)

			w := &responseTester{
				header:             http.Header{},
				expectedStatusCode: http.StatusOK,
				expectedBody:       ":\ntopics: https://example.com/foos/b\nid: b\ndata: d2\n\n",
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
				expectedBody:       ":\ntopics: https://example.com/foos/b\nid: b\ndata: d2\n\n",
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
			ID:     "b",
			Data:   "d2",
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
			ID:     "a", Data: "d1",
		}))
		// Private event the anonymous subscriber is NOT authorized to
		// read. Its id must not appear in the Last-Event-ID response.
		require.NoError(t, transport.Dispatch(t.Context(), &Update{
			Topics:  []string{"https://example.com/foos/b"},
			Private: true,
			ID:      "b", Data: "secret",
		}))

		ctx := t.Context()

		go func(ctx context.Context) {
			c, cancel := context.WithCancel(ctx)
			req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match_urlpattern=https://example.com/foos/:id&last_event_id=unknown", nil).WithContext(c)

			w := &responseTester{
				header:             http.Header{},
				expectedStatusCode: http.StatusOK,
				expectedBody:       ":\ntopics: https://example.com/foos/c\nid: c\ndata: d3\n\n",
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
			ID:     "c", Data: "d3",
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
				expectedBody:       ":\ntopics: https://example.com/foos/b\nid: b\ndata: d2\n\n",
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
				expectedBody:       ":\ntopics: https://example.com/foos/b\nid: b\ndata: d2\n\n",
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
			ID:     "b",
			Data:   "d2",
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
				expectedBody:       ":\ntopics: https://example.com/foo\nid: e1\ndata: d\n\n",
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
			ID:     "e1", Data: "d",
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
				Data:   "Hello World", ID: "b",
			})

			return
		}
	}()

	ctx, cancel := context.WithCancel(ctx)
	req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match=https://example.com/books/1&match_urlpattern=https://example.com/reviews/:id", nil).WithContext(ctx)

	w := &responseTester{
		expectedStatusCode: http.StatusOK,
		expectedBody:       ":\ntopics: https://example.com/books/1\nid: b\ndata: Hello World\n\n:\n",
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
		Issuer:               testIssuer,
		Audience:             jwt.ClaimStrings{testResourceIdentifier},
		ExpiresAt:            jwt.NewNumericDate(time.Now().Add(time.Second)),
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
				s.Claims = &claims{
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(tc.tokenExpiresIn)),
				}
			}

			rc := h.newResponseController(httptest.NewRecorder(), s)

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
	rc := h.newResponseController(httptest.NewRecorder(), &LocalSubscriber{})

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

	subs := sseSubscriptions(t, body)
	require.NotEmpty(t, subs)
	assert.True(t, slices.ContainsFunc(subs, func(sub subscription) bool {
		return sub.Active && sub.Match == "/.well-known/mercure/subscriptions/:mt/:m/:s"
	}), "the subscriber was not told about its own subscription")
}

type topicFieldsRequest struct {
	method, query, body string
	wantTopics          bool
}

func TestSubscribeTopics(t *testing.T) {
	t.Parallel()

	requests := map[string]topicFieldsRequest{
		"exact":      {method: http.MethodGet, query: "match=https://example.com/books/1", wantTopics: true},
		"urlpattern": {method: http.MethodGet, query: "match_urlpattern=https://example.com/books/:id", wantTopics: true},
		"query_body": {method: methodQuery, body: "match=https://example.com/books/1", wantTopics: true},
	}
	for name, request := range requests {
		for _, compatibility := range []int{0, 7, 8} {
			for _, private := range []bool{false, true} {
				for _, replay := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/compatibility=%d/private=%t/replay=%t", name, compatibility, private, replay), func(t *testing.T) {
						t.Parallel()

						synctest.Test(t, func(t *testing.T) {
							testSubscribeTopics(t, compatibility, private, replay, request)
						})
					})
				}
			}
		}
	}
}

func testSubscribeTopics(t *testing.T, compatibility int, private, replay bool, request topicFieldsRequest) {
	t.Helper()

	var transport Transport = NewLocalTransport(NewSubscriberList(0))
	if replay {
		transport = createBoltTransport(t, 0, 0)
	}

	options := []Option{WithTransport(transport)}
	if compatibility != 0 {
		options = append(options, WithProtocolVersionCompatibility(compatibility))
	}

	hub := createDummy(t, options...)

	update := &Update{
		Topics: []string{
			"https://example.com/books/1",
			"https://example.com/users/42/books/1",
			"https://example.com/users/7/books/1",
			"https://example.com/users/42/inbox/1",
		},
		Data: "Foo", ID: "a", Private: private,
	}
	if replay {
		require.NoError(t, transport.Dispatch(t.Context(), update))
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// Routing selects the canonical topic; authorization selects only alternates.
	req := httptest.NewRequest(request.method, defaultHubURL+"?"+request.query, strings.NewReader(request.body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", bearerPrefix+createDummySubscriberJWTWithDetails(t, nil,
		TopicMatcher{Type: MatcherTypeURLPattern, Pattern: "https://example.com/users/42/*"}))

	if replay {
		req.Header.Set("Last-Event-ID", EarliestLastEventID)
	}

	w := newSubscribeRecorder()

	go hub.SubscribeHandler(w, req)

	synctest.Wait()

	if !replay {
		require.NoError(t, transport.Dispatch(t.Context(), update))
		synctest.Wait()
	}

	cancel()
	synctest.Wait()

	var fields string

	if request.wantTopics {
		if private {
			fields = "topics: https://example.com/users/42/books/1\ntopics: https://example.com/users/42/inbox/1\n"
		} else {
			fields = "topics: https://example.com/books/1\ntopics: https://example.com/users/42/books/1\ntopics: https://example.com/users/7/books/1\ntopics: https://example.com/users/42/inbox/1\n"
		}
	}

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, fields+"id: a\ndata: Foo\n\n", w.Body.String())
}
