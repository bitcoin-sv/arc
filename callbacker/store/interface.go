package store

import (
	"context"
	"errors"

	"github.com/bitcoin-sv/arc/callbacker/callbacker_api"
)

var (
	ErrNotFound   = errors.New("key could not be found")
	ErrMaxRetries = errors.New("max retries reached")
)

type Store interface {
	Get(ctx context.Context, key string) (*callbacker_api.Callback, error)
	GetExpired(context.Context) (map[string]*callbacker_api.Callback, error)
	Set(ctx context.Context, callback *callbacker_api.Callback) (string, error)
	UpdateExpiry(ctx context.Context, key string) error
	Del(ctx context.Context, key string) error
	Close(context.Context) error
}
