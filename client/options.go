package client

import (
	"github.com/TAAL-GmbH/arc/config"
	"github.com/coocood/freecache"
	"github.com/mrz1836/go-cache"
	"github.com/mrz1836/go-cachestore"
	"github.com/mrz1836/go-datastore"
)

type Options func(c *clientOptions)

type (
	Client struct {
		options *clientOptions
	}

	clientOptions struct {
		cachestore        cachestore.ClientInterface
		cachestoreOptions []cachestore.ClientOps
		datastore         datastore.ClientInterface
		datastoreOptions  datastore.ClientOps
		metamorphs        []string
		migrateModels     []interface{}
		minerID           *config.MinerIDConfig
		nodes             []TransactionHandler
		nodesOptions      []*config.NodeConfig
	}
)

// WithDatastore will set the Datastore
func WithDatastore(config *config.DatastoreConfig) Options {
	if config != nil {
		if config.Engine == datastore.SQLite {
			return WithSQLite(config.SQLite)
		} else if config.Engine == datastore.MySQL || config.Engine == datastore.Postgres {
			return WithSQL(config.Engine, []*datastore.SQLConfig{config.SQL})
		} else if config.Engine == datastore.MongoDB {
			return WithMongoDB(config.Mongo)
		}
	}

	return nil
}

// WithMigrateModels will set the models to auto migrate on the database
func WithMigrateModels(models []interface{}) Options {
	return func(c *clientOptions) {
		if models != nil {
			c.migrateModels = models
		}
	}
}

// WithMongoDB will set the Datastore to use MongoDB
func WithMongoDB(config *datastore.MongoDBConfig) Options {
	return func(c *clientOptions) {
		if config != nil {
			c.datastoreOptions = datastore.WithMongo(config)
		}
	}
}

// WithSQL will set the Datastore to use SQL (Postgres, MySQL, MS SQL)
func WithSQL(engine datastore.Engine, config []*datastore.SQLConfig) Options {
	return func(c *clientOptions) {
		if config != nil {
			c.datastoreOptions = datastore.WithSQL(engine, config)
		}
	}
}

// WithSQLite will set the Datastore to use SQLite
func WithSQLite(config *datastore.SQLiteConfig) Options {
	return func(c *clientOptions) {
		if config != nil {
			c.datastoreOptions = datastore.WithSQLite(config)
		}
	}
}

// WithNodeConfig sets nodes from a config
func WithNodeConfig(nodesConfig []*config.NodeConfig) Options {
	return func(o *clientOptions) {
		o.nodesOptions = nodesConfig
	}
}

// WithNode sets a single node to use in calls
func WithNode(node TransactionHandler) Options {
	return func(o *clientOptions) {
		o.nodes = []TransactionHandler{
			node,
		}
	}
}

// WithNodes sets multiple nodes to use in calls
func WithNodes(nodes []TransactionHandler) Options {
	return func(o *clientOptions) {
		o.nodes = []TransactionHandler{}
		for _, node := range nodes {
			addNode := node
			o.nodes = append(o.nodes, addNode)
		}
	}
}

// WithMetamorphs sets all metamorph servers to use
func WithMetamorphs(servers []string) Options {
	return func(o *clientOptions) {
		o.metamorphs = servers
	}
}

// WithMinerID sets the miner ID to use in all calls
func WithMinerID(conf *config.MinerIDConfig) Options {
	return func(o *clientOptions) {
		o.minerID = conf
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
