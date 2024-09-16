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

func (c GrpcCallbacker) SendCallback(tx *store.StoreData) {
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

func toGrpcInput(d *store.StoreData) *callbacker_api.SendCallbackRequest {
	endpoints := make([]*callbacker_api.CallbackEndpoint, 0, len(d.Callbacks))

	for _, c := range d.Callbacks {
		if c.CallbackURL != "" {
			endpoints = append(endpoints, &callbacker_api.CallbackEndpoint{
				Url:   c.CallbackURL,
				Token: c.CallbackToken,
			})
		}
	}

	if len(endpoints) == 0 {
		return nil
	}

	in := callbacker_api.SendCallbackRequest{
		CallbackEndpoints: endpoints,

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

func getCallbackExtraInfo(d *store.StoreData) string {
	if d.Status == metamorph_api.Status_MINED && len(d.CompetingTxs) > 0 {
		return minedDoubleSpendMsg
	}

	return d.RejectReason
}

func getCallbackCompetitingTxs(d *store.StoreData) []string {
	if d.Status == metamorph_api.Status_MINED {
		return nil
	}

	return d.CompetingTxs
}

func getCallbackBlockHash(d *store.StoreData) string {
	if d.BlockHash == nil {
		return ""
	}

	return d.BlockHash.String()
}
