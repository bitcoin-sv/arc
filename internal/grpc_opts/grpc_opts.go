package grpc_opts

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"runtime/debug"

	"github.com/bitcoin-sv/arc/internal/grpc_opts/common_api"
	"github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	prometheusclient "github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	arc_logger "github.com/bitcoin-sv/arc/internal/logger"
)

var ErrGRPCFailedToRegisterPanics = fmt.Errorf("failed to register panics total metric")

func GetGRPCServerOpts(logger *slog.Logger, prometheusEndpoint string, grpcMessageSize int, service string) (*prometheus.ServerMetrics, []grpc.ServerOption, func(), error) {
	// Setup logging.
	rpcLogger := logger.With(slog.String("service", "gRPC/server"))

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
		Name: fmt.Sprintf("grpc_req_panics_recovered_%s_total", service),
		Help: "Total number of gRPC requests recovered from internal panic.",
	})

	err := prometheusclient.Register(panicsTotal)
	if err != nil {
		return nil, nil, nil, errors.Join(ErrGRPCFailedToRegisterPanics, err)
	}

	grpcPanicRecoveryHandler := func(p any) (err error) {
		panicsTotal.Inc()
		rpcLogger.Error("recovered from panic", "panic", p, "stack", debug.Stack())
		return status.Errorf(codes.Internal, "%s", p)
	}

	var chainUnaryInterceptors []grpc.UnaryServerInterceptor

	if prometheusEndpoint != "" {
		chainUnaryInterceptors = append(chainUnaryInterceptors, srvMetrics.UnaryServerInterceptor(prometheus.WithExemplarFromContext(exemplarFromContext)))
	}

	chainUnaryInterceptors = append(chainUnaryInterceptors, // Order matters e.g. tracing interceptor have to create span first for the later exemplars to work.
		recovery.UnaryServerInterceptor(recovery.WithRecoveryHandler(grpcPanicRecoveryHandler)))

	// decorate context with event ID
	chainUnaryInterceptors = append(chainUnaryInterceptors, func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		if event, ok := req.(common_api.UnaryEvent); ok && event != nil {
			id := event.GetEventId()
			if id != "" {
				ctx = context.WithValue(ctx, arc_logger.EventIDField, event.GetEventId()) //nolint:staticcheck // ignore SA1029
			}
		}

		return handler(ctx, req)
	})

	opts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(chainUnaryInterceptors...),
		grpc.MaxRecvMsgSize(grpcMessageSize),
	}

	cleanup := func() {
		prometheusclient.Unregister(panicsTotal)
	}

	return srvMetrics, opts, cleanup, err
}

func GetGRPCClientOpts(prometheusEndpoint string, grpcMessageSize int) ([]grpc.DialOption, error) {

	clientMetrics := prometheus.NewClientMetrics(
		prometheus.WithClientHandlingTimeHistogram(
			prometheus.WithHistogramBuckets([]float64{0.001, 0.01, 0.1, 0.3, 0.6, 1, 3, 6, 9, 20, 30, 60, 90, 120}),
		),
	)

	exemplarFromContext := func(ctx context.Context) prometheusclient.Labels {
		if span := trace.SpanContextFromContext(ctx); span.IsSampled() {
			return prometheusclient.Labels{"traceID": span.TraceID().String()}
		}
		return nil
	}

	var chainUnaryInterceptors []grpc.UnaryClientInterceptor

	if prometheusEndpoint != "" {
		chainUnaryInterceptors = append(chainUnaryInterceptors, clientMetrics.UnaryClientInterceptor(prometheus.WithExemplarFromContext(exemplarFromContext)))
	}

	// add eventID from context to grpc request if possible
	chainUnaryInterceptors = append(chainUnaryInterceptors, func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if eventID, ok := ctx.Value(arc_logger.EventIDField).(string); ok {
			trySetEventId(req, eventID)
		}

		return invoker(ctx, method, req, reply, cc, opts...)
	})

	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(chainUnaryInterceptors...),
		grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin":{}}]}`), // This sets the initial balancing policy.
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(grpcMessageSize)),
	}

	return dialOpts, nil
}

func trySetEventId(x any, eventId string) {
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

	field.SetString(eventId)
}
