package grpc_opts

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"runtime/debug"

	"github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	prometheusclient "github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/internal/grpc_opts/common_api"

	arc_logger "github.com/bitcoin-sv/arc/internal/logger"
)

var ErrGRPCFailedToRegisterPanics = fmt.Errorf("failed to register panics total metric")

func GetGRPCServerOpts(logger *slog.Logger, prometheusEndpoint string, grpcMessageSize int, service string, tracingConfig *config.TracingConfig) (*prometheus.ServerMetrics, []grpc.ServerOption, func(), error) {
	// Setup logging.
	rpcLogger := logger.With(slog.String("service", "gRPC/server"))

	// Setup metrics.
	srvMetrics := prometheus.NewServerMetrics(
		prometheus.WithServerHandlingTimeHistogram(
			prometheus.WithHistogramBuckets([]float64{0.001, 0.01, 0.1, 0.3, 0.6, 1, 3, 6, 9, 20, 30, 60, 90, 120}),
		),
	)

	// Setup metric for panic recoveries.
	panicsTotal := prometheusclient.NewCounter(prometheusclient.CounterOpts{
		Name: fmt.Sprintf("grpc_req_panics_recovered_%s_total", service),
		Help: "Total number of gRPC requests recovered from internal panic.",
	})

	err := prometheusclient.Register(panicsTotal)
	if err != nil {
		return nil, nil, nil, errors.Join(ErrGRPCFailedToRegisterPanics, err)
	}
	opts := make([]grpc.ServerOption, 0)

	if tracingConfig != nil && tracingConfig.IsEnabled() {
		opts = append(opts, grpc.StatsHandler(otelgrpc.NewServerHandler()))
	}

	grpcPanicRecoveryHandler := func(p any) (err error) {
		panicsTotal.Inc()
		rpcLogger.Error("recovered from panic", "panic", p, "stack", debug.Stack())
		return status.Errorf(codes.Internal, "%s", p)
	}

	var chainUnaryInterceptors []grpc.UnaryServerInterceptor

	if prometheusEndpoint != "" {
		exemplarFromContext := func(ctx context.Context) prometheusclient.Labels {
			if span := trace.SpanContextFromContext(ctx); span.IsSampled() {
				return prometheusclient.Labels{"traceID": span.TraceID().String()}
			}
			return nil
		}
		chainUnaryInterceptors = append(chainUnaryInterceptors, srvMetrics.UnaryServerInterceptor(prometheus.WithExemplarFromContext(exemplarFromContext)))
	}

	chainUnaryInterceptors = append(chainUnaryInterceptors, // Order matters e.g. tracing interceptor have to create span first for the later exemplars to work.
		recovery.UnaryServerInterceptor(recovery.WithRecoveryHandler(grpcPanicRecoveryHandler)))

	// decorate context with event ID
	chainUnaryInterceptors = append(chainUnaryInterceptors, func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		if event, ok := req.(common_api.UnaryEvent); ok && event != nil {
			id := event.GetEventId()
			if id != "" {
				//nolint:staticcheck // use string key on purpose
				ctx = context.WithValue(ctx, arc_logger.EventIDField, event.GetEventId()) //lint:ignore SA1029 use string key on purpose
			}
		}

		return handler(ctx, req)
	})

	opts = append(opts, grpc.ChainUnaryInterceptor(chainUnaryInterceptors...))
	opts = append(opts, grpc.MaxRecvMsgSize(grpcMessageSize))

	cleanup := func() {
		prometheusclient.Unregister(panicsTotal)
	}

	return srvMetrics, opts, cleanup, err
}

func GetGRPCClientOpts(prometheusEndpoint string, grpcMessageSize int, tracingConfig *config.TracingConfig) ([]grpc.DialOption, error) {
	clientMetrics := prometheus.NewClientMetrics(
		prometheus.WithClientHandlingTimeHistogram(
			prometheus.WithHistogramBuckets([]float64{0.001, 0.01, 0.1, 0.3, 0.6, 1, 3, 6, 9, 20, 30, 60, 90, 120}),
		),
	)
	opts := make([]grpc.DialOption, 0)

	if tracingConfig != nil && tracingConfig.IsEnabled() {
		opts = append(opts, grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
	}

	var chainUnaryInterceptors []grpc.UnaryClientInterceptor

	if prometheusEndpoint != "" {
		exemplarFromContext := func(ctx context.Context) prometheusclient.Labels {
			if span := trace.SpanContextFromContext(ctx); span.IsSampled() {
				return prometheusclient.Labels{"traceID": span.TraceID().String()}
			}
			return nil
		}
		chainUnaryInterceptors = append(chainUnaryInterceptors, clientMetrics.UnaryClientInterceptor(prometheus.WithExemplarFromContext(exemplarFromContext)))
	}

	// add eventID from context to grpc request if possible
	chainUnaryInterceptors = append(chainUnaryInterceptors, func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if eventID, ok := ctx.Value(arc_logger.EventIDField).(string); ok {
			trySetEventID(req, eventID)
		}

		return invoker(ctx, method, req, reply, cc, opts...)
	})

	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	opts = append(opts, grpc.WithChainUnaryInterceptor(chainUnaryInterceptors...))
	opts = append(opts, grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin":{}}]}`)) // This sets the initial balancing policy.
	opts = append(opts, grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(grpcMessageSize)))

	return opts, nil
}

func trySetEventID(x any, eventID string) {
	// get the value and type of the argument
	v := reflect.ValueOf(x)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	// if x is not a struct return
	if v.Kind() != reflect.Struct {
		return
	}

	// check if struct has EventId field
	field := v.FieldByName("EventId")
	if !field.IsValid() && !field.CanSet() && field.Kind() != reflect.String {
		return
	}

	field.SetString(eventID)
}
