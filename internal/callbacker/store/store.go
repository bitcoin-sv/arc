package store

import (
	"context"
	"errors"
	"time"
)

var (
	ErrURLMappingsDeleteFailed = errors.New("failed to delete URL mapping entries")
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
	DeleteOlderThan(ctx context.Context, t time.Time) error
	SetMany(ctx context.Context, data []*CallbackData) (int64, error)
	GetMany(ctx context.Context, limit int, expiration time.Duration, batch bool) ([]*CallbackData, error)
	SetSent(ctx context.Context, ids []int64) error
	SetNotPending(ctx context.Context, ids []int64) error
}
