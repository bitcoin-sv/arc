package store

import (
	"context"
	"errors"
	"time"
)

var (
	ErrURLMappingDuplicateKey  = errors.New("URL mapping duplicate key")
	ErrURLMappingDeleteFailed  = errors.New("failed to delete URL mapping entry")
	ErrURLMappingsDeleteFailed = errors.New("failed to delete URL mapping entries")
	ErrNoUnmappedURLsFound     = errors.New("no unmapped URLs found")
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
	SetURLMapping(ctx context.Context, m URLMapping) error
	GetURLMappings(ctx context.Context) (urlInstanceMappings map[string]string, err error)
	DeleteURLMapping(ctx context.Context, instance string) (rowsAffected int64, err error)
	GetUnmappedURL(ctx context.Context) (url string, err error)
	DeleteOlderThan(ctx context.Context, t time.Time) error
	SetMany(ctx context.Context, data []*CallbackData) error
}

type CallbackStore interface {
	DeleteURLMappingsExcept(ctx context.Context, except []string) (rowsAffected int64, err error)
}

type URLMapping struct {
	URL      string
	Instance string
}
