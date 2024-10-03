package metamorph

import (
	"context"
	"log/slog"

	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
)

type GrpcCallbacker struct {
	cc callbacker_api.CallbackerAPIClient
	l  *slog.Logger
}

func NewGrpcCallbacker(api callbacker_api.CallbackerAPIClient, logger *slog.Logger) GrpcCallbacker {
	return GrpcCallbacker{
		cc: api,
		l:  logger,
	}
}

func (c GrpcCallbacker) SendCallback(tx *store.Data) {
	if len(tx.Callbacks) == 0 {
		return
	}

	in := toGrpcInput(tx)
	if in == nil {
		return
	}

	_, err := c.cc.SendCallback(context.Background(), in)
	if err != nil {
		c.l.Error("sending callback failed", slog.String("err", err.Error()), slog.Any("input", in))
	}
}

func toGrpcInput(d *store.Data) *callbacker_api.SendCallbackRequest {
	routings := make([]*callbacker_api.CallbackRouting, 0, len(d.Callbacks))

	for _, c := range d.Callbacks {
		if c.CallbackURL != "" {
			routings = append(routings, &callbacker_api.CallbackRouting{
				Url:        c.CallbackURL,
				Token:      c.CallbackToken,
				AllowBatch: c.AllowBatch,
			})
		}
	}

	if len(routings) == 0 {
		return nil
	}

	in := callbacker_api.SendCallbackRequest{
		CallbackRoutings: routings,

		Txid:         d.Hash.String(),
		Status:       callbacker_api.Status(d.Status),
		MerklePath:   d.MerklePath,
		ExtraInfo:    getCallbackExtraInfo(d),
		CompetingTxs: getCallbackCompetitingTxs(d),

		BlockHash:   getCallbackBlockHash(d),
		BlockHeight: d.BlockHeight,
	}

	return &in
}

func getCallbackExtraInfo(d *store.Data) string {
	if d.Status == metamorph_api.Status_MINED && len(d.CompetingTxs) > 0 {
		return minedDoubleSpendMsg
	}

	return d.RejectReason
}

func getCallbackCompetitingTxs(d *store.Data) []string {
	if d.Status == metamorph_api.Status_MINED {
		return nil
	}

	return d.CompetingTxs
}

func getCallbackBlockHash(d *store.Data) string {
	if d.BlockHash == nil {
		return ""
	}

	return d.BlockHash.String()
}
