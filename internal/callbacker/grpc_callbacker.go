package callbacker

import (
	"context"
	"log/slog"
	"runtime"

	"go.opentelemetry.io/otel/attribute"

	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
	"github.com/bitcoin-sv/arc/internal/global"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/pkg/tracing"
)

var minedDoubleSpendMsg = "previously double spend attempted"

type GrpcCallbacker struct {
	client            callbacker_api.CallbackerAPIClient
	logger            *slog.Logger
	tracingEnabled    bool
	tracingAttributes []attribute.KeyValue
}

func WithTracerCallbacker(attr ...attribute.KeyValue) func(*GrpcCallbacker) {
	return func(p *GrpcCallbacker) {
		p.tracingEnabled = true
		if len(attr) > 0 {
			p.tracingAttributes = append(p.tracingAttributes, attr...)
		}
		_, file, _, ok := runtime.Caller(1)
		if ok {
			p.tracingAttributes = append(p.tracingAttributes, attribute.String("file", file))
		}
	}
}

type Option func(s *GrpcCallbacker)

func NewGrpcCallbacker(api callbacker_api.CallbackerAPIClient, logger *slog.Logger, opts ...Option) GrpcCallbacker {
	c := GrpcCallbacker{
		client: api,
		logger: logger,
	}

	for _, opt := range opts {
		opt(&c)
	}

	return c
}

func (c GrpcCallbacker) SendCallback(ctx context.Context, data *global.Data) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "SendCallback", c.tracingEnabled, c.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if len(data.Callbacks) == 0 {
		return
	}

	inputs := toGrpcInputs(data)
	if inputs == nil {
		return
	}

	for _, in := range inputs {
		_, err = c.client.SendCallback(ctx, in)
		if err != nil {
			c.logger.Error("sending callback failed", slog.String("err", err.Error()), slog.Any("input", in))
		}
	}
}

func toGrpcInputs(data *global.Data) []*callbacker_api.SendRequest {
	requests := make([]*callbacker_api.SendRequest, 0, len(data.Callbacks))

	for _, c := range data.Callbacks {
		if c.CallbackURL != "" {
			in := callbacker_api.SendRequest{
				CallbackRouting: &callbacker_api.CallbackRouting{
					Url:        c.CallbackURL,
					Token:      c.CallbackToken,
					AllowBatch: c.AllowBatch,
				},
				Txid:         data.Hash.String(),
				Status:       callbacker_api.Status(data.Status),
				MerklePath:   data.MerklePath,
				ExtraInfo:    getCallbackExtraInfo(data),
				CompetingTxs: getCallbackCompetitingTxs(data),

				BlockHash:   getCallbackBlockHash(data),
				BlockHeight: data.BlockHeight,
			}

			requests = append(requests, &in)
		}
	}

	return requests
}

func getCallbackExtraInfo(data *global.Data) string {
	if data.Status == metamorph_api.Status_MINED && len(data.CompetingTxs) > 0 {
		return minedDoubleSpendMsg
	}

	return data.RejectReason
}

func getCallbackCompetitingTxs(data *global.Data) []string {
	if data.Status == metamorph_api.Status_MINED {
		return nil
	}

	return data.CompetingTxs
}

func getCallbackBlockHash(data *global.Data) string {
	if data.BlockHash == nil {
		return ""
	}

	return data.BlockHash.String()
}
