package mercure

import (
	"context"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "github.com/dunglas/mercure"

// startSpan starts a span using the TracerProvider attached to the active span
// in ctx. When no tracer is active (e.g. Caddy's `tracing` directive is not
// enabled), this falls back to the OpenTelemetry no-op tracer: no exporters,
// no globals touched.
func startSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	tracer := trace.SpanFromContext(ctx).TracerProvider().Tracer(tracerName)

	return tracer.Start(ctx, name, opts...)
}

// recordSpanError marks the span as errored.
func recordSpanError(span trace.Span, err error) {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}
