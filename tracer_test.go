package mercure

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// spanRecorder returns a context whose active span is backed by an in-memory
// SpanRecorder. The library's startSpan helper pulls the TracerProvider out of
// the context, so any Mercure operation invoked with this context will have
// its spans captured by the returned recorder.
func spanRecorder(t *testing.T) (context.Context, *tracetest.SpanRecorder) {
	t.Helper()

	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))

	ctx, root := tp.Tracer("test").Start(context.Background(), "root")

	t.Cleanup(func() { root.End() })

	return ctx, sr
}

func endedSpanNames(sr *tracetest.SpanRecorder) []string {
	spans := sr.Ended()
	names := make([]string, len(spans))

	for i, s := range spans {
		names[i] = s.Name()
	}

	return names
}

func TestPublishEmitsSpans(t *testing.T) {
	ctx, sr := spanRecorder(t)

	hub := createAnonymousDummy(t)

	require.NoError(t, hub.Publish(ctx, &Update{
		Topics: []string{"https://example.com/books/1"},
		Event:  Event{Data: "hello"},
	}))

	assert.Contains(t, endedSpanNames(sr), "mercure.publish")
}

func TestSubscribeEmitsSpan(t *testing.T) {
	ctx, sr := spanRecorder(t)

	hub := createAnonymousDummy(t)

	reqCtx, cancel := context.WithCancel(ctx)
	req := httptest.NewRequest(
		http.MethodGet,
		defaultHubURL+"?topic=https://example.com/books/1",
		nil,
	).WithContext(reqCtx)

	// responseTester cancels the request as soon as the initial ":\n" comment
	// reaches the client — by that point registerSubscriber has returned and
	// its deferred span.End has fired, so the span is in sr.Ended().
	w := &responseTester{
		expectedStatusCode: http.StatusOK,
		expectedBody:       ":\n",
		tb:                 t,
		cancel:             cancel,
	}

	hub.SubscribeHandler(w, req)

	assert.Contains(t, endedSpanNames(sr), "mercure.subscribe")
}

func TestSubscriptionsEmitsSpan(t *testing.T) {
	ctx, sr := spanRecorder(t)

	hub := createAnonymousDummy(t, WithSubscriptions())

	req := httptest.NewRequest(http.MethodGet, subscriptionsURL, nil).WithContext(ctx)
	w := httptest.NewRecorder()

	hub.SubscriptionsHandler(w, req)

	assert.Contains(t, endedSpanNames(sr), "mercure.subscriptions")
}

func TestBoltHistoryEmitsSpan(t *testing.T) {
	ctx, sr := spanRecorder(t)

	transport := createBoltTransport(t, 0, 0)

	topics := []string{"https://example.com/books/1"}
	for i := 1; i <= 3; i++ {
		require.NoError(t, transport.Dispatch(ctx, &Update{
			Event:  Event{ID: strconv.Itoa(i)},
			Topics: topics,
		}))
	}

	// A subscriber with a RequestLastEventID forces AddSubscriber to replay
	// history, which is the span we want to verify.
	s := NewLocalSubscriber("1", transport.logger, &TopicSelectorStore{})
	s.SetTopics(topics, nil)

	require.NoError(t, transport.AddSubscriber(ctx, s))

	assert.Contains(t, endedSpanNames(sr), "mercure.transport.history")
}
