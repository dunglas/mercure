//go:build deprecated_topic && deprecated_claim

package mercure

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSubscribeDeprecatedTopicParam covers the v8 end-to-end flow: a
// URI-template selector in the `topic` query parameter still matches under
// WithProtocolVersionCompatibility(8).
func TestSubscribeDeprecatedTopicParam(t *testing.T) {
	t.Parallel()

	hub := createDeprecatedDummy(t, WithAnonymous())

	go func() {
		s, _ := hub.transport.(*LocalTransport)

		var ready bool

		for !ready {
			s.RLock()
			ready = s.subscribers.Len() != 0
			s.RUnlock()
		}

		_ = hub.transport.Dispatch(t.Context(), &Update{
			Topics: []string{"https://example.com/books/1"},
			Data:   "Hello World", ID: "a",
		})
	}()

	ctx, cancel := context.WithCancel(t.Context())
	req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?topic=https://example.com/books/{id}", nil).WithContext(ctx)

	w := &responseTester{
		expectedStatusCode: http.StatusOK,
		expectedBody:       ":\nid: a\ndata: Hello World\n\n",
		tb:                 t,
		cancel:             cancel,
	}

	hub.SubscribeHandler(w, req)
}

// 0.x clients replay history with their own cursor parameters, and get the
// 0.x Last-Event-ID response field, even when reconnecting with only the
// Last-Event-ID request header.
func TestSubscribeDeprecatedTopicParamLastEventID(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		transport := createBoltTransport(t, 0, 0)
		hub := createDummy(t, WithAnonymous(), WithLogger(transport.logger), WithTransport(transport), WithProtocolVersionCompatibility(7))

		require.NoError(t, transport.Dispatch(t.Context(), &Update{
			Topics: []string{"https://example.com/books/1"},
			ID:     "a", Data: "d1",
		}))
		require.NoError(t, transport.Dispatch(t.Context(), &Update{
			Topics: []string{"https://example.com/books/2"},
			ID:     "b", Data: "d2",
		}))

		for _, tc := range []struct{ query, header, lastEventID, expectedBody string }{
			{query: "&lastEventID=a", lastEventID: "a", expectedBody: ":\nid: b\ndata: d2\n\n"},
			{query: "&lastEventID=earliest", lastEventID: EarliestLastEventID, expectedBody: ":\nid: a\ndata: d1\n\nid: b\ndata: d2\n\n"},
			{query: "&Last-Event-ID=a", lastEventID: "a", expectedBody: ":\nid: b\ndata: d2\n\n"},
			{header: "a", lastEventID: "a", expectedBody: ":\nid: b\ndata: d2\n\n"},
			// Some 0.x clients also send last_event_id, which takes precedence.
			{query: "&last_event_id=a", lastEventID: "a", expectedBody: ":\nid: b\ndata: d2\n\n"},
			{query: "&Last-Event-ID=a&last_event_id=a", lastEventID: "a", expectedBody: ":\nid: b\ndata: d2\n\n"},
			{query: "&lastEventID=earliest&last_event_id=a", lastEventID: "a", expectedBody: ":\nid: b\ndata: d2\n\n"},
			// Like 0.x, an empty lastEventID requests no history.
			{query: "&lastEventID=", expectedBody: ":\n"},
		} {
			go func() {
				ctx, cancel := context.WithCancel(t.Context())
				req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?topic=https://example.com/books/{id}"+tc.query, nil).WithContext(ctx)

				if tc.header != "" {
					req.Header.Set("Last-Event-ID", tc.header)
				}

				w := &responseTester{
					header:             http.Header{},
					expectedStatusCode: http.StatusOK,
					expectedBody:       tc.expectedBody,
					tb:                 t,
					cancel:             cancel,
				}

				hub.SubscribeHandler(w, req)
				assert.Equal(t, tc.lastEventID, w.Header().Get("Mercure-Last-Event-ID"))
				assert.Equal(t, tc.lastEventID, w.Header().Get("Last-Event-ID"))
			}()
		}

		synctest.Wait()
	})
}

// A URI template too complex to compile is refused up front instead of matching nothing.
func TestSubscribeDeprecatedTopicParamTooComplex(t *testing.T) {
	t.Parallel()

	hub := createDeprecatedDummy(t, WithAnonymous())

	topic := "https://example.com/{" + strings.Repeat("a,", 1100) + "a}"
	req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?topic="+url.QueryEscape(topic), nil)
	w := httptest.NewRecorder()
	hub.SubscribeHandler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
}
