package cache

import (
	"errors"
	"time"
)

var (
	ErrCacheNotFound         = errors.New("key not found in cache")
	ErrCacheFailedToSet      = errors.New("failed to set value in cache")
	ErrCacheFailedToDel      = errors.New("failed to delete value from cache")
	ErrCacheFailedToGet      = errors.New("failed to get value from cache")
	ErrCacheFailedToScan     = errors.New("failed to scan cache")
	ErrCacheFailedToGetCount = errors.New("failed to get count from cache")
)

type Store interface {
	Get(hash *string, key string) ([]byte, error)
	GetAllForHash(hash string) (map[string][]byte, error)
	Set(hash *string, key string, value []byte, ttl time.Duration) error
	Del(hash *string, keys ...string) error
	CountElementsForHash(hash string) (int64, error)
}
