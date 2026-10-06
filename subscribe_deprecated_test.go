//go:build deprecated_topic && deprecated_claim

package mercure

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
