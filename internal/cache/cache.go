package cache

import (
	"errors"
	"time"
)

var (
	ErrCacheNotFound          = errors.New("key not found in cache")
	ErrCacheFailedToSet       = errors.New("failed to set value in cache")
	ErrCacheFailedToDel       = errors.New("failed to delete value from cache")
	ErrCacheFailedToGet       = errors.New("failed to get value from cache")
	ErrCacheFailedToScan      = errors.New("failed to scan cache")
	ErrCacheFailedToGetCount  = errors.New("failed to get count from cache")
	ErrCacheFailedToExecuteTx = errors.New("failed to execute transaction")
)

type Store interface {
	Get(key string) ([]byte, error)
	Set(key string, value []byte, ttl time.Duration) error
	Del(keys ...string) error

	MapGet(hash string, key string) ([]byte, error)
	MapGetAll(hash string) (map[string][]byte, error)
	MapSet(hash string, key string, value []byte) error
	MapDel(hash string, keys ...string) error
	MapLen(hash string) (int64, error)
	MapExtractAll(hash string) (map[string][]byte, error)
}
