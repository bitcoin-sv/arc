package callbacker

import (
	"context"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
)

func sendRequestToDto(r *callbacker_api.SendRequest) *Callback {
	dto := Callback{
		TxID:      r.Txid,
		TxStatus:  r.Status.String(),
		Timestamp: time.Now().UTC(),
	}

	if r.BlockHash != "" {
		dto.BlockHash = ptrTo(r.BlockHash)
		dto.BlockHeight = ptrTo(r.BlockHeight)
	}

	if r.MerklePath != "" {
		dto.MerklePath = ptrTo(r.MerklePath)
	}

	if r.ExtraInfo != "" {
		dto.ExtraInfo = ptrTo(r.ExtraInfo)
	}

	if len(r.CompetingTxs) > 0 {
		dto.CompetingTxs = r.CompetingTxs
	}

	return &dto
}

type JetstreamMsg interface {
	Metadata() (*jetstream.MsgMetadata, error)
	Data() []byte
	Headers() nats.Header
	Subject() string
	Reply() string
	Ack() error
	DoubleAck(context.Context) error
	Nak() error
	NakWithDelay(delay time.Duration) error
	InProgress() error
	Term() error
	TermWithReason(reason string) error
}
