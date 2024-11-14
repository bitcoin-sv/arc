package cache

import (
	"errors"
	"time"
)

var (
	ErrCacheNotFound             = errors.New("key not found in cache")
	ErrCacheFailedToSet          = errors.New("failed to set value in cache")
	ErrCacheFailedToDel          = errors.New("failed to delete value from cache")
	ErrCacheFailedToGet          = errors.New("failed to get value from cache")
	ErrCacheFailedToMarshalValue = errors.New("failed to marshal value")
)

type Store interface {
	Get(key string) ([]byte, error)
	Set(key string, value []byte, ttl time.Duration) error
	Del(key string) error
}
