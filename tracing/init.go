package tracing

import (
	"fmt"
	"io"

	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/config"
)

// InitTracer creates a new instance of Jaeger tracer.
func InitTracer(serviceName string) (opentracing.Tracer, io.Closer, error) {
	cfg, err := config.FromEnv()
	if err != nil {
		return nil, nil, fmt.Errorf("cannot parse jaeger env vars: %v", err.Error())
	}

	cfg.ServiceName = serviceName
	cfg.Sampler.Type = jaeger.SamplerTypeConst
	cfg.Sampler.Param = 100

	var tracer opentracing.Tracer
	var closer io.Closer
	tracer, closer, err = cfg.NewTracer()
	if err != nil {

		return nil, nil, fmt.Errorf("cannot initialize jaeger tracer: %v", err.Error())
	}

	return tracer, closer, nil
}
