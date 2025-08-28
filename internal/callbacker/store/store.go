package store

import (
	"context"
	"time"
)

type CallbackData struct {
	ID           int64
	URL          string
	Token        string
	Timestamp    time.Time
	CompetingTxs []string
	TxID         string
	TxStatus     string
	ExtraInfo    *string
	MerklePath   *string
	BlockHash    *string
	BlockHeight  *uint64
	AllowBatch   bool
}

type ProcessorStore interface {
	Clear(ctx context.Context, t time.Time) error
	Insert(ctx context.Context, data []*CallbackData) (int64, error)
	GetUnsent(ctx context.Context, limit int, expiration time.Duration, batch bool) ([]*CallbackData, error)
	SetSent(ctx context.Context, ids []int64) error
	UnsetPending(ctx context.Context, ids []int64) error
}
