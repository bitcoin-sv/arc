package client

import (
	"github.com/TAAL-GmbH/mapi/bitcoin"
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
		nodes            []*bitcoin.Node
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
func WithNode(node *bitcoin.Node) Options {
	return func(o *clientOptions) {
		o.nodes = []*bitcoin.Node{
			node,
		}
	}
}

// WithNodes sets multiple nodes to use in calls
func WithNodes(nodes []bitcoin.Node) Options {
	return func(o *clientOptions) {
		o.nodes = []*bitcoin.Node{}
		for _, node := range nodes {
			addNode := node
			o.nodes = append(o.nodes, &addNode)
		}
	}
}

// WithBitcoinRestServer sets a single node to use in calls
func WithBitcoinRestServer(server string) Options {
	return func(o *clientOptions) {
		node := bitcoin.NewRest(server)
		o.nodes = []*bitcoin.Node{
			&node,
		}
	}
}

// WithBitcoinRestServers sets multiple nodes to use in calls
func WithBitcoinRestServers(servers []string) Options {
	return func(o *clientOptions) {
		o.nodes = []*bitcoin.Node{}
		for _, nodeEndpoint := range servers {
			node := bitcoin.NewRest(nodeEndpoint)
			o.nodes = append(o.nodes, &node)
		}
	}
}

// WithBitcoinRPCServer sets a single node to use in calls
func WithBitcoinRPCServer(config *config.RPCClientConfig) Options {
	return func(o *clientOptions) {
		node := bitcoin.NewRPC(config)
		o.nodes = []*bitcoin.Node{
			&node,
		}
	}
}

// WithBitcoinRPCSServers sets multiple rpc servers to use in calls
func WithBitcoinRPCSServers(servers []string) Options {
	return func(o *clientOptions) {
		o.nodes = []*bitcoin.Node{}
		for _, nodeEndpoint := range servers {
			node := bitcoin.NewRest(nodeEndpoint)
			o.nodes = append(o.nodes, &node)
		}
	}
}

// WithMinerID sets the miner ID to use in all calls
func WithMinerID(config *config.MinerIDConfig) Options {
	return func(o *clientOptions) {
		o.minerID = config
	}
}
