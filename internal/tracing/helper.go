package tracing

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func StartTracing(ctx context.Context, spanName string, tracingEnabled bool, attributes ...attribute.KeyValue) (context.Context, trace.Span) {
	if tracingEnabled {
		var span trace.Span
		tracer := otel.Tracer("")
		if tracer == nil {
			return ctx, nil
		}

		if len(attributes) > 0 {
			ctx, span = tracer.Start(ctx, spanName, trace.WithAttributes(attributes...))
			return ctx, span
		}

		ctx, span = tracer.Start(ctx, spanName)
		return ctx, span
	}
	return ctx, nil
}

func EndTracing(span trace.Span) {
	if span != nil {
		span.End()
	}
}
