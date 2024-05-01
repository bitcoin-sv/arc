package grpc_server

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"

	"github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	prometheusclient "github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// interceptorLogger adapts slog logger to interceptor logger.
func interceptorLogger(l *slog.Logger) logging.Logger {
	return logging.LoggerFunc(func(_ context.Context, lvl logging.Level, msg string, fields ...any) {
		switch lvl {
		case logging.LevelDebug:
			l.Debug(msg, fields...)
		case logging.LevelInfo:
			l.Info(msg, fields...)
		case logging.LevelWarn:
			l.Warn(msg, fields...)
		case logging.LevelError:
			l.Error(msg, fields...)
		default:
			panic(fmt.Sprintf("unknown level %v", lvl))
		}
	})
}

func GetGRPCServerOpts(logger *slog.Logger, prometheusEndpoint string, grpcMessageSize int) (*prometheus.ServerMetrics, []grpc.ServerOption, func(), error) {
	// Setup logging.
	rpcLogger := logger.With(slog.String("service", "gRPC/server"))
	logTraceID := func(ctx context.Context) logging.Fields {
		if span := trace.SpanContextFromContext(ctx); span.IsSampled() {
			return logging.Fields{"traceID", span.TraceID().String()}
		}
		return nil
	}

	// Setup metrics.
	srvMetrics := prometheus.NewServerMetrics(
		prometheus.WithServerHandlingTimeHistogram(
			prometheus.WithHistogramBuckets([]float64{0.001, 0.01, 0.1, 0.3, 0.6, 1, 3, 6, 9, 20, 30, 60, 90, 120}),
		),
	)

	exemplarFromContext := func(ctx context.Context) prometheusclient.Labels {
		if span := trace.SpanContextFromContext(ctx); span.IsSampled() {
			return prometheusclient.Labels{"traceID": span.TraceID().String()}
		}
		return nil
	}

	// Setup metric for panic recoveries.
	panicsTotal := prometheusclient.NewCounter(prometheusclient.CounterOpts{
		Name: "grpc_req_panics_recovered_total",
		Help: "Total number of gRPC requests recovered from internal panic.",
	})

	err := prometheusclient.Register(panicsTotal)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to register panics total metric: %w", err)
	}

	grpcPanicRecoveryHandler := func(p any) (err error) {
		panicsTotal.Inc()
		rpcLogger.Error("recovered from panic", "panic", p, "stack", debug.Stack())
		return status.Errorf(codes.Internal, "%s", p)
	}

	var chainUnaryInterceptors []grpc.UnaryServerInterceptor
	var chainStreamInterceptors []grpc.StreamServerInterceptor

	if prometheusEndpoint != "" {
		chainUnaryInterceptors = append(chainUnaryInterceptors, srvMetrics.UnaryServerInterceptor(prometheus.WithExemplarFromContext(exemplarFromContext)))
		chainStreamInterceptors = append(chainStreamInterceptors, srvMetrics.StreamServerInterceptor(prometheus.WithExemplarFromContext(exemplarFromContext)))
	}

	chainUnaryInterceptors = append(chainUnaryInterceptors, // Order matters e.g. tracing interceptor have to create span first for the later exemplars to work.
		logging.UnaryServerInterceptor(interceptorLogger(rpcLogger), logging.WithFieldsFromContext(logTraceID)),
		recovery.UnaryServerInterceptor(recovery.WithRecoveryHandler(grpcPanicRecoveryHandler)))
	chainStreamInterceptors = append(chainStreamInterceptors, srvMetrics.StreamServerInterceptor(prometheus.WithExemplarFromContext(exemplarFromContext)),
		logging.StreamServerInterceptor(interceptorLogger(rpcLogger), logging.WithFieldsFromContext(logTraceID)),
		recovery.StreamServerInterceptor(recovery.WithRecoveryHandler(grpcPanicRecoveryHandler)))
	opts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(chainUnaryInterceptors...),
		grpc.ChainStreamInterceptor(chainStreamInterceptors...),
		grpc.MaxRecvMsgSize(grpcMessageSize),
	}

	cleanup := func() {
		prometheusclient.Unregister(panicsTotal)
	}

	return srvMetrics, opts, cleanup, err
}
