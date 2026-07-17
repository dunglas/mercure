//go:build deprecated_topic && deprecated_claim

package mercure

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
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
			Topic: "https://example.com/books/1",
			Event: Event{Data: "Hello World", ID: "a"},
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
