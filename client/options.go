package client

import (
	"github.com/TAAL-GmbH/arc/api"
	"github.com/TAAL-GmbH/arc/config"
	"github.com/coocood/freecache"
	"github.com/mrz1836/go-cache"
	"github.com/mrz1836/go-cachestore"
)

type Options func(c *clientOptions)

type (
	Client struct {
		options *clientOptions
	}

	clientOptions struct {
		cachestore         cachestore.ClientInterface
		cachestoreOptions  []cachestore.ClientOps
		fees               []api.Fee
		metamorphs         []string
		nodesOptions       []*config.NodeConfig
		transactionHandler TransactionHandler
	}
)

// WithNode sets a single node to use in calls
func WithNode(transactionHandler TransactionHandler) Options {
	return func(o *clientOptions) {
		o.transactionHandler = transactionHandler
	}
}

// WithNodeConfig sets nodes from a config
func WithNodeConfig(nodesConfig []*config.NodeConfig) Options {
	return func(o *clientOptions) {
		o.nodesOptions = nodesConfig
	}
}

// WithMetamorphs sets all metamorph servers to use
func WithMetamorphs(servers []string) Options {
	return func(o *clientOptions) {
		o.metamorphs = servers
	}
}

// WithCustomCachestore will set the cachestore
func WithCustomCachestore(cacheStore cachestore.ClientInterface) Options {
	return func(c *clientOptions) {
		if cacheStore != nil {
			c.cachestore = cacheStore
		}
	}
}

// WithFreeCache will set the cache client for both Read & Write clients
func WithFreeCache() Options {
	return func(c *clientOptions) {
		c.cachestoreOptions = append(c.cachestoreOptions, cachestore.WithFreeCache())
	}
}

// WithFreeCacheConnection will set the cache client to an active FreeCache connection
func WithFreeCacheConnection(client *freecache.Cache) Options {
	return func(c *clientOptions) {
		if client != nil {
			c.cachestoreOptions = append(
				c.cachestoreOptions,
				cachestore.WithFreeCacheConnection(client),
			)
		}
	}
}

// WithRedis will set the redis cache client for both Read & Write clients
//
// This will load new redis connections using the given parameters
func WithRedis(config *config.RedisConfig) Options {
	return func(c *clientOptions) {
		if config != nil {
			c.cachestoreOptions = append(c.cachestoreOptions, cachestore.WithRedis(&cachestore.RedisConfig{
				DependencyMode:        config.DependencyMode,
				MaxActiveConnections:  config.MaxActiveConnections,
				MaxConnectionLifetime: config.MaxConnectionLifetime,
				MaxIdleConnections:    config.MaxIdleConnections,
				MaxIdleTimeout:        config.MaxIdleTimeout,
				URL:                   config.URL,
				UseTLS:                config.UseTLS,
			}))
		}
	}
}

// WithRedisConnection will set the cache client to an active redis connection
func WithRedisConnection(activeClient *cache.Client) Options {
	return func(c *clientOptions) {
		if activeClient != nil {
			c.cachestoreOptions = append(c.cachestoreOptions, cachestore.WithRedisConnection(activeClient))
		}
	}
}
