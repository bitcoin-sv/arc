package client

import (
	"github.com/TAAL-GmbH/mapi/config"
	"github.com/mrz1836/go-datastore"
)

type Options func(c *clientOptions)

type (
	Client struct {
		options *clientOptions
	}

	clientOptions struct {
		datastore        datastore.ClientInterface
		datastoreOptions datastore.ClientOps
		migrateModels    []interface{}
		minerID          *config.MinerIDConfig
		nodes            []TransactionHandler
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

// WithMinerID sets the miner ID to use in all calls
func WithMinerID(conf *config.MinerIDConfig) Options {
	return func(o *clientOptions) {
		o.minerID = conf
	}
}
