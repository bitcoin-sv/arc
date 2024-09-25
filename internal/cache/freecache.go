package cache

import (
	"errors"
	"time"

	"github.com/coocood/freecache"
)

// FreecacheStore is an implementation of CacheStore using freecache.
type FreecacheStore struct {
	cache *freecache.Cache
}

// NewFreecacheStore initializes a FreecacheStore.
func NewFreecacheStore(c *freecache.Cache) *FreecacheStore {
	return &FreecacheStore{
		cache: c,
	}
}

// Get retrieves a value by key.
func (f *FreecacheStore) Get(key string) ([]byte, error) {
	value, err := f.cache.Get([]byte(key))
	if err != nil {
		if errors.Is(err, freecache.ErrNotFound) {
			return nil, ErrCacheNotFound
		}
		return nil, err
	}
	return value, nil
}

// Set stores a value with a TTL.
func (f *FreecacheStore) Set(key string, value []byte, ttl time.Duration) error {
	err := f.cache.Set([]byte(key), value, int(ttl.Seconds()))
	if err != nil {
		return err
	}
	return nil
}

// Del removes a value by key.
func (f *FreecacheStore) Del(key string) error {
	success := f.cache.Del([]byte(key))
	if !success {
		return ErrCacheNotFound
	}
	return nil
}
