package store

import (
	"context"
	"time"
)

type CallbackerStore interface {
	SetMany(ctx context.Context, data []*CallbackData) error
	PopMany(ctx context.Context, limit int) ([]*CallbackData, error)
	Close() error
}

type CallbackData struct {
	Url   string
	Token string

	Timestamp time.Time

	CompetingTxs []string

	TxID       string
	TxStatus   string
	ExtraInfo  *string
	MerklePath *string

	BlockHash   *string
	BlockHeight *uint64
}
