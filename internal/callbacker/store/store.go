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

	AllowBatch bool
}

type ProcessorStore interface {
	SetURLMapping(ctx context.Context, m URLMapping) error
	GetURLMappings(ctx context.Context) (urlInstanceMappings map[string]string, err error)
	DeleteURLMapping(ctx context.Context, instance string) error
	GetUnmappedURL(ctx context.Context) (url string, err error)
	GetAndDelete(ctx context.Context, url string, limit int) ([]*CallbackData, error)
	DeleteOlderThan(ctx context.Context, t time.Time) error
}

type CallbackStore interface {
	DeleteURLMapping(ctx context.Context, instance string) error
}

type URLMapping struct {
	URL      string
	Instance string
}
