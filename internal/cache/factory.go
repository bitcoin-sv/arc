package cache

import (
	"errors"
	"github.com/bitcoin-sv/arc/config"
)

var ErrCacheUnknownType = errors.New("unknown cache type")

// NewCacheStore creates a new CacheStore based on the provided configuration.
func NewCacheStore(cacheConfig *config.CacheConfig) (Store, error) {
	switch cacheConfig.Engine {
	case config.FreeCache:
		return NewFreecacheStore(cacheConfig.Freecache.Size), nil
	case config.Redis:
		return NewRedisStore(cacheConfig.Redis.Addr, cacheConfig.Redis.Password, cacheConfig.Redis.DB), nil
	default:
		return nil, ErrCacheUnknownType
	}
}
