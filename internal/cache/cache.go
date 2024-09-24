package cache

import (
	"errors"
	"time"
)

var ErrCacheNotFound = errors.New("key not found in cache")

type Store interface {
	Get(key string) ([]byte, error)
	Set(key string, value []byte, ttl time.Duration) error
	Del(key string) error
}
