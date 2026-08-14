package mercure

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseEventsDuration(t *testing.T) {
	t.Parallel()

	for header, expected := range map[string]struct {
		duration time.Duration
		ok       bool
	}{
		"duration=600":        {600 * time.Second, true},
		"duration=0.5":        {500 * time.Millisecond, true},
		"foo=1, duration=10":  {10 * time.Second, true},
		"duration=10;param=1": {10 * time.Second, true},
		"duration=0":          {0, false},
		"duration=-5":         {0, false},
		"duration=NaN":        {0, false},
		"duration=Inf":        {0, false},
		"duration=oops":       {0, false},
		"other=1":             {0, false},
		"":                    {0, false},
		`duration="600"`:      {0, false},
		"Duration=600":        {0, false},
	} {
		d, ok := parseEventsDuration(header)
		assert.Equal(t, expected.ok, ok, header)
		assert.Equal(t, expected.duration, d, header)
	}
}

func TestSSEEncoder(t *testing.T) {
	t.Parallel()

	enc := sseEncoder{}

	assert.Equal(t, "text/event-stream", enc.contentType())
	assert.Equal(t, ":\n", string(enc.preamble()))
	assert.Equal(t, ":\n", string(enc.heartbeat()))
	assert.Nil(t, enc.trailer())

	payload, err := enc.encode(&Update{Event: Event{ID: "i", Data: "d", Type: "t", Retry: 3}})
	require.NoError(t, err)
	assert.Equal(t, "event: t\nretry: 3\nid: i\ndata: d\n\n", payload)
}

func TestMultipartEncoder(t *testing.T) {
	t.Parallel()

	enc := newMultipartEncoder()

	assert.Nil(t, enc.preamble())
	assert.Nil(t, enc.heartbeat())

	mediaType, params, err := mime.ParseMediaType(enc.contentType())
	require.NoError(t, err)
	assert.Equal(t, "multipart/mixed", mediaType)
	require.NotEmpty(t, params["boundary"])

	// Data passes through unescaped; SSE-specific properties (type, retry)
	// are not represented in this encoding.
	first, err := enc.encode(&Update{
		ContentType: "application/ld+json",
		Event:       Event{ID: "urn:uuid:first", Data: "{\"line\": 1}\n{\"line\": 2}", Type: "t", Retry: 3},
	})
	require.NoError(t, err)

	second, err := enc.encode(&Update{Event: Event{ID: "urn:uuid:second", Data: "Hello World"}})
	require.NoError(t, err)

	trailer := enc.trailer()
	require.NotNil(t, trailer)
	assert.True(t, strings.HasSuffix(string(trailer), "--"+params["boundary"]+"--\r\n"))

	mr := multipart.NewReader(strings.NewReader(first+second+string(trailer)), params["boundary"])

	part, err := mr.NextPart()
	require.NoError(t, err)
	assert.Equal(t, "<urn:uuid:first>", part.Header.Get("Content-Id"))
	assert.Equal(t, "application/ld+json", part.Header.Get("Content-Type"))

	body, err := io.ReadAll(part)
	require.NoError(t, err)
	assert.Equal(t, "{\"line\": 1}\n{\"line\": 2}", string(body))
	assert.Equal(t, strconv.Itoa(len(body)), part.Header.Get("Content-Length"))
	assert.Empty(t, part.Header.Values("Event"))

	part, err = mr.NextPart()
	require.NoError(t, err)
	assert.Equal(t, "<urn:uuid:second>", part.Header.Get("Content-Id"))
	assert.Empty(t, part.Header.Get("Content-Type"))

	body, err = io.ReadAll(part)
	require.NoError(t, err)
	assert.Equal(t, "Hello World", string(body))

	_, err = mr.NextPart()
	require.ErrorIs(t, err, io.EOF)
}

func TestNegotiateStreamEncoder(t *testing.T) {
	t.Parallel()

	enabled := createAnonymousDummy(t, WithEventsQuery())
	disabled := createAnonymousDummy(t)

	for _, tc := range []struct {
		name        string
		hub         *Hub
		method      string
		accept      string
		isMultipart bool
	}{
		{"disabled", disabled, methodQuery, "multipart/mixed", false},
		{"get", enabled, http.MethodGet, "multipart/mixed", false},
		{"no accept", enabled, methodQuery, "", false},
		{"sse", enabled, methodQuery, "text/event-stream", false},
		{"multipart", enabled, methodQuery, "multipart/mixed", true},
		{"quality ordering", enabled, methodQuery, "text/event-stream;q=0.5, multipart/mixed", true},
		{"unsupported falls back", enabled, methodQuery, "application/xml", false},
		{"wildcard falls back", enabled, methodQuery, "*/*", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(tc.method, defaultHubURL, nil)
			if tc.accept != "" {
				req.Header.Set("Accept", tc.accept)
			}

			_, isMultipart := tc.hub.negotiateStreamEncoder(req).(*multipartEncoder)
			assert.Equal(t, tc.isMultipart, isMultipart)
		})
	}
}

// containsResponseTester cancels the request context once the accumulated
// body contains the expected fragment; unlike responseTester it does not
// exact-match, so it fits multipart bodies whose boundary is random.
type containsResponseTester struct {
	header             http.Header
	body               string
	expectedStatusCode int
	contains           string
	cancel             context.CancelFunc
	tb                 testing.TB
}

func (rt *containsResponseTester) Header() http.Header {
	return rt.header
}

func (rt *containsResponseTester) Write(buf []byte) (int, error) {
	rt.body += string(buf)

	if strings.Contains(rt.body, rt.contains) {
		rt.cancel()
	}

	return len(buf), nil
}

func (rt *containsResponseTester) WriteHeader(statusCode int) {
	assert.Equal(rt.tb, rt.expectedStatusCode, statusCode)
}

func (rt *containsResponseTester) Flush() {
}

func (rt *containsResponseTester) SetWriteDeadline(_ time.Time) error {
	return nil
}

func TestSubscribeEventsQueryMultipart(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t, WithEventsQuery())
	transport, _ := hub.transport.(*LocalTransport)
	ctx := t.Context()

	go func() {
		for {
			transport.RLock()
			ready := transport.subscribers.Len() == 1
			transport.RUnlock()

			if !ready {
				continue
			}

			_ = hub.transport.Dispatch(ctx, &Update{
				Topics:      []string{"https://example.com/books/1"},
				ContentType: "application/ld+json",
				Event:       Event{Data: "Hello World", ID: "b"},
			})

			return
		}
	}()

	reqCtx, cancel := context.WithCancel(t.Context())
	body := url.Values{"match": {"https://example.com/books/1"}, "last_event_id": {EarliestLastEventID}}.Encode()
	req := httptest.NewRequest(methodQuery, defaultHubURL, strings.NewReader(body)).WithContext(reqCtx)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "multipart/mixed")
	req.Header.Set("Events", "duration=300")

	w := &containsResponseTester{
		header:             http.Header{},
		expectedStatusCode: http.StatusOK,
		contains:           "Hello World",
		cancel:             cancel,
		tb:                 t,
	}
	hub.SubscribeHandler(w, req)

	mediaType, params, err := mime.ParseMediaType(w.Header().Get("Content-Type"))
	require.NoError(t, err)
	assert.Equal(t, "multipart/mixed", mediaType)

	assert.Equal(t, "?1", w.Header().Get("Incremental"))
	assert.Equal(t, "application/x-www-form-urlencoded", w.Header().Get("Accept-Query"))
	assert.Equal(t, EarliestLastEventID, w.Header().Get("Mercure-Last-Event-Id"))

	duration, ok := parseEventsDuration(w.Header().Get("Events"))
	require.True(t, ok)
	assert.LessOrEqual(t, duration, 300*time.Second)

	// The client disconnected, so no close delimiter was written: append one
	// to parse the body as a complete multipart document.
	boundary := params["boundary"]
	mr := multipart.NewReader(strings.NewReader(w.body+"\r\n--"+boundary+"--\r\n"), boundary)

	part, err := mr.NextPart()
	require.NoError(t, err)
	assert.Equal(t, "<b>", part.Header.Get("Content-Id"))
	assert.Equal(t, "application/ld+json", part.Header.Get("Content-Type"))

	partBody, err := io.ReadAll(part)
	require.NoError(t, err)
	assert.Equal(t, "Hello World", string(partBody))
}

// Subscription events flow through the shared registration path, so they
// reach negotiated encodings like any other update.
func TestSubscribeEventsQuerySubscriptionEvents(t *testing.T) {
	t.Parallel()

	hub := createDummy(t, WithEventsQuery(), WithSubscriptions())

	reqCtx, cancel := context.WithCancel(t.Context())
	body := url.Values{"match_urlpattern": {"/.well-known/mercure/subscriptions/*"}}.Encode()
	req := httptest.NewRequest(methodQuery, defaultHubURL, strings.NewReader(body)).WithContext(reqCtx)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "multipart/mixed")
	req.AddCookie(&http.Cookie{Name: defaultCookieName, Value: createDummySubscriberJWTWithDetails(t, nil, TopicMatcher{Type: MatcherTypeURLPattern, Pattern: "/.well-known/mercure/subscriptions/*"})})

	w := &containsResponseTester{
		header:             http.Header{},
		expectedStatusCode: http.StatusOK,
		contains:           `"active": true`,
		cancel:             cancel,
		tb:                 t,
	}
	hub.SubscribeHandler(w, req)

	_, params, err := mime.ParseMediaType(w.Header().Get("Content-Type"))
	require.NoError(t, err)

	boundary := params["boundary"]
	mr := multipart.NewReader(strings.NewReader(w.body+"\r\n--"+boundary+"--\r\n"), boundary)

	part, err := mr.NextPart()
	require.NoError(t, err)
	assert.NotEmpty(t, part.Header.Get("Content-Id"))

	partBody, err := io.ReadAll(part)
	require.NoError(t, err)
	assert.Contains(t, string(partBody), `"active": true`)
	assert.Contains(t, string(partBody), `"type": "subscription"`)
}

// A clean hub-side disconnection ends a multipart stream with the close
// delimiter, and a client-provided duration caps the connection lifetime.
func TestSubscribeEventsQueryDurationAndTrailer(t *testing.T) {
	t.Parallel()

	// The disconnection timer fires one dispatchTimeout before the
	// client-requested bound, leaving room to write the close delimiter
	// before the recorder's deadline.
	hub := createAnonymousDummy(t, WithEventsQuery(), WithWriteTimeout(600*time.Second), WithDispatchTimeout(time.Second), WithHeartbeat(0))

	reqCtx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	body := url.Values{"match": {"https://example.com/books/1"}}.Encode()
	req := httptest.NewRequest(methodQuery, defaultHubURL, strings.NewReader(body)).WithContext(reqCtx)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "multipart/mixed")
	req.Header.Set("Events", "duration=1.5")

	w := newSubscribeRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() {
		require.NoError(t, resp.Body.Close())
	})

	duration, ok := parseEventsDuration(resp.Header.Get("Events"))
	require.True(t, ok)
	assert.LessOrEqual(t, duration, 2*time.Second)

	_, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	require.NoError(t, err)
	assert.Equal(t, "\r\n--"+params["boundary"]+"--\r\n", w.Body.String())
}

// With the feature enabled, an SSE subscriber can also bound the response
// duration; the stream itself stays text/event-stream.
func TestSubscribeSSEDuration(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t, WithEventsQuery(), WithWriteTimeout(600*time.Second), WithDispatchTimeout(0), WithHeartbeat(0))

	reqCtx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match=https://example.com/books/1", nil).WithContext(reqCtx)
	req.Header.Set("Events", "duration=1")

	w := newSubscribeRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() {
		require.NoError(t, resp.Body.Close())
	})

	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	assert.Empty(t, resp.Header.Get("Incremental"))

	duration, ok := parseEventsDuration(resp.Header.Get("Events"))
	require.True(t, ok)
	assert.LessOrEqual(t, duration, time.Second)
}

// With the feature disabled, the Events request header is ignored and no
// events query response headers leak.
func TestSubscribeEventsHeaderIgnoredWhenDisabled(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t)

	ctx, cancel := context.WithCancel(t.Context())
	req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match=https://example.com/books/1", nil).WithContext(ctx)
	req.Header.Set("Events", "duration=1")

	go func() {
		transport, _ := hub.transport.(*LocalTransport)

		for {
			transport.RLock()
			ready := transport.subscribers.Len() == 1
			transport.RUnlock()

			if ready {
				cancel()

				return
			}
		}
	}()

	w := &responseTester{
		header:             http.Header{},
		expectedStatusCode: http.StatusOK,
		expectedBody:       ":\n",
		tb:                 t,
		cancel:             func() {},
	}
	hub.SubscribeHandler(w, req)

	assert.Empty(t, w.Header().Get("Events"))
	assert.Empty(t, w.Header().Get("Accept-Query"))
	assert.Empty(t, w.Header().Get("Incremental"))
}
