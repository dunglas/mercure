package mercure

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestPublishEmitsSpans(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))

	// Seed the context with a span whose TracerProvider is the recorder; the
	// library pulls the tracer from ctx via trace.SpanFromContext.
	ctx, root := tp.Tracer("test").Start(context.Background(), "root")
	defer root.End()

	hub := createAnonymousDummy(t)

	require.NoError(t, hub.Publish(ctx, &Update{
		Topics: []string{"https://example.com/books/1"},
		Event:  Event{Data: "hello"},
	}))

	spans := sr.Ended()
	require.Len(t, spans, 1)
	assert.Equal(t, "mercure.publish", spans[0].Name())
}
