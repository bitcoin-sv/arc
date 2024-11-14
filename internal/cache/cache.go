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
	GetAllWithPrefix(prefix string) (map[string][]byte, error)
	Set(key string, value []byte, ttl time.Duration) error
	Del(keys ...string) error

	MapGet(hashsetKey string, field string) ([]byte, error)
	MapGetAll(hashsetKey string) (map[string][]byte, error)
	MapSet(hashsetKey string, field string, value []byte) error
	MapDel(hashsetKey string, fields ...string) error
	MapLen(hashsetKey string) (int64, error)
	MapExtractAll(hashsetKey string) (map[string][]byte, error)
}
