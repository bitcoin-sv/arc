package store

import (
	"context"
	"time"
)

type CallbackerStore interface {
	Set(ctx context.Context, dto *CallbackData) error
	SetMany(ctx context.Context, data []*CallbackData) error
	PopMany(ctx context.Context, limit int) ([]*CallbackData, error)
	PopFailedMany(ctx context.Context, t time.Time, limit int) ([]*CallbackData, error) // TODO: lepsza nazwa dla t
	DeleteFailedOlderThan(ctx context.Context, t time.Time) error
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

	PostponedUntil *time.Time
}
