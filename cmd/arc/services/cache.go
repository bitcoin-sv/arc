package cmd

import (
	"context"
	"errors"
	"github.com/bitcoin-sv/arc/internal/cache"

	"github.com/bitcoin-sv/arc/config"
	"github.com/coocood/freecache"
	"github.com/go-redis/redis/v8"
)

var ErrCacheUnknownType = errors.New("unknown cache type")

// NewCacheStore creates a new CacheStore based on the provided configuration.
func NewCacheStore(cacheConfig *config.CacheConfig) (cache.Store, error) {
	switch cacheConfig.Engine {
	case config.FreeCache:
		cacheSize := freecache.NewCache(cacheConfig.Freecache.Size)
		return cache.NewFreecacheStore(cacheSize), nil
	case config.Redis:
		c := redis.NewClient(&redis.Options{
			Addr:     cacheConfig.Redis.Addr,
			Password: cacheConfig.Redis.Password,
			DB:       cacheConfig.Redis.DB,
		})
		return cache.NewRedisStore(context.Background(), c), nil
	default:
		return nil, ErrCacheUnknownType
	}
}
