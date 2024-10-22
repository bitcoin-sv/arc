package metamorph

import (
	"context"
	"go.opentelemetry.io/otel/trace"
)

var tracer trace.Tracer

// WithTracer sets the tracer to be used for tracing
func WithTracer(t trace.Tracer) {
	tracer = t
}

// StartTracing starts a new span with the given name
func StartTracing(spanName string) trace.Span {
	if tracer != nil {
		_, span := tracer.Start(context.Background(), spanName)
		return span
	}
	return nil
}

// EndTracing ends the given span
func EndTracing(span trace.Span) {
	if span != nil {
		span.End()
	}
}
