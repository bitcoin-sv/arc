package store

import (
	"context"
	"errors"
	"time"
)

var (
	ErrURLMappingDuplicateKey = errors.New("URL mapping duplicate key")
	ErrURLMappingDeleteFailed = errors.New("failed to delete URL mapping entry")
)

type CallbackerStore interface {
	Set(ctx context.Context, dto *CallbackData) error
	SetMany(ctx context.Context, data []*CallbackData) error
	PopMany(ctx context.Context, limit int) ([]*CallbackData, error)
	PopFailedMany(ctx context.Context, t time.Time, limit int) ([]*CallbackData, error) // TODO: find better name
	DeleteFailedOlderThan(ctx context.Context, t time.Time) error
	Close() error
}

type CallbackData struct {
	URL   string
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
	AllowBatch     bool
}

type ProcessorStore interface {
	SetURLMapping(ctx context.Context, m URLMapping) error
	GetURLMappings(ctx context.Context) (urlInstanceMappings map[string]string, err error)
	DeleteURLMapping(ctx context.Context, instance string) error
}

type URLMapping struct {
	URL      string
	Instance string
}
