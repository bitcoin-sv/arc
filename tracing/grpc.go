package tracing

import (
	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/opentracing/opentracing-go"
	"google.golang.org/grpc"
)

func AddGRPCDialOptions(opts []grpc.DialOption) []grpc.DialOption {
	tracer := opentracing.GlobalTracer()
	if tracer != nil {
		opts = append(opts, grpc.WithUnaryInterceptor(otgrpc.OpenTracingClientInterceptor(tracer)))
		opts = append(opts, grpc.WithStreamInterceptor(otgrpc.OpenTracingStreamClientInterceptor(tracer)))
	}

	return opts
}

func AddGRPCServerOptions(opts []grpc.ServerOption) []grpc.ServerOption {
	tracer := opentracing.GlobalTracer()
	if tracer != nil {
		opts = append(opts, grpc.UnaryInterceptor(otgrpc.OpenTracingServerInterceptor(tracer)))
		opts = append(opts, grpc.StreamInterceptor(otgrpc.OpenTracingStreamServerInterceptor(tracer)))
	}

	return opts
}
