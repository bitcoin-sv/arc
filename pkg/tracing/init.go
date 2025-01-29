package tracing

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func NewTraceProvider(ctx context.Context, serviceName string, sample int, opts ...otlptracegrpc.Option) (*trace.TracerProvider, *otlptrace.Exporter, error) {
	exporter, err := otlptracegrpc.New(
		ctx,
		opts...,
	)
	if err != nil {
		return nil, nil, err
	}

	traceOpt := trace.WithSampler(trace.AlwaysSample())
	if sample != 0 && sample != 100 {
		traceOpt = trace.WithSampler(trace.TraceIDRatioBased(float64(sample) / 100))
	}

	tp := trace.NewTracerProvider(
		trace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
		)),
		trace.WithBatcher(exporter),
		traceOpt,
	)

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	otel.SetTracerProvider(tp)

	return tp, exporter, nil
}

func Enable(logger *slog.Logger, serviceName string, dialAddr string, sample int) (func(), error) {
	if dialAddr == "" {
		return nil, errors.New("tracing enabled, but tracing address empty")
	}

	ctx := context.Background()

	tp, exporter, err := NewTraceProvider(ctx, serviceName, sample, otlptracegrpc.WithEndpointURL(dialAddr), otlptracegrpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("failed to create trace provider: %v", err)
	}

	cleanup := func() {
		err = exporter.Shutdown(ctx)
		if err != nil {
			logger.Error("Failed to shutdown exporter", slog.String("err", err.Error()))
		}

		err = tp.Shutdown(ctx)
		if err != nil {
			logger.Error("Failed to shutdown tracing provider", slog.String("err", err.Error()))
		}
	}

	return cleanup, nil
}
