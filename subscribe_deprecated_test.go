//go:build deprecated_topic && deprecated_claim

package mercure

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/synctest"
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

func TestSubscribeTopicsLegacyRequests(t *testing.T) {
	t.Parallel()

	requests := map[string]topicFieldsRequest{
		"legacy":            {method: http.MethodGet, query: "topic=https://example.com/books/{id}"},
		"legacy_query_body": {method: methodQuery, body: "topic=https://example.com/books/{id}"},
		// The modern matcher opts in even when only the legacy matcher selects the update.
		"mixed":            {method: http.MethodGet, query: "topic=https://example.com/books/{id}&match=https://example.com/other", wantTopics: true},
		"mixed_query_body": {method: methodQuery, query: "topic=https://example.com/books/{id}", body: "match_urlpattern=https://example.com/other/:id", wantTopics: true},
	}
	for name, request := range requests {
		for _, compatibility := range []int{7, 8} {
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
