package client

import (
	"context"
	"math/rand"
	"time"

	"github.com/TAAL-GmbH/arc/dictionary"
	"github.com/mrz1836/go-cachestore"
	"github.com/mrz1836/go-datastore"
	"github.com/mrz1836/go-logger"
)

type Interface interface {
	Close()
	Load(ctx context.Context) (err error)
	Cachestore() cachestore.ClientInterface
	Datastore() datastore.ClientInterface
	GetMinerID() (minerID string)
	GetNode(index int) TransactionHandler
	GetNodes() []TransactionHandler
	GetRandomNode() TransactionHandler
	Models() []interface{}
}

func New(opts ...Options) (Interface, error) {
	client := &Client{
		&clientOptions{
			nodes: []TransactionHandler{},
		},
	}

	// Overwrite defaults with any custom options provided by the user
	for _, opt := range opts {
		opt(client.options)
	}

	if len(client.options.nodes) == 0 {
		// default to localhost node, if none configured
		node, err := NewBitcoinNode("localhost", 8332, "", "", false)
		if err != nil {
			return nil, err
		}
		client.options.nodes = append(client.options.nodes, node)
	}

	if err := client.Load(context.Background()); err != nil {
		return nil, err
	}

	return client, nil
}

func (c *Client) Close() {
	_ = c.Datastore().Close(context.Background())
}

func (c *Client) GetNode(index int) TransactionHandler {
	if len(c.options.nodes) < index-1 {
		return nil
	}

	return c.options.nodes[index]
}

func (c *Client) GetNodes() []TransactionHandler {
	return c.options.nodes
}

func (c *Client) GetRandomNode() TransactionHandler {
	if len(c.options.nodes) > 1 {
		rand.Seed(time.Now().Unix())
		k := rand.Intn(len(c.options.nodes)) //nolint:gosec - no need to be perfect here
		return c.options.nodes[k]
	}

	return c.options.nodes[0]
}

// Load all services in the Client
func (c *Client) Load(ctx context.Context) (err error) {
	if err = c.loadDatastore(ctx); err != nil {
		return err
	}

	if err = c.loadCachestore(ctx); err != nil {
		return err
	}

	if err = c.loadTransactionHandler(ctx); err != nil {
		return err
	}

	return
}

func (c *Client) loadDatastore(ctx context.Context) (err error) {
	if c.options.datastoreOptions != nil {
		var opts []datastore.ClientOps
		opts = append(opts, c.options.datastoreOptions)
		if len(c.Models()) > 0 {
			// this will create the base tables from the gorm definition
			opts = append(opts, datastore.WithAutoMigrate(c.Models()...))
		}
		if c.options.datastore, err = datastore.NewClient(ctx, opts...); err != nil {
			return err
		}
	} else if c.options.datastore == nil {
		// we initialize a SQL lite db by default, if no other database connection has been set
		var opts []datastore.ClientOps
		opts = append(opts, datastore.WithSQLite(&datastore.SQLiteConfig{
			DatabasePath: "./arc.db",
			Shared:       true,
		}))
		if len(c.Models()) > 0 {
			// this will create the base tables from the gorm definition
			opts = append(opts, datastore.WithAutoMigrate(c.Models()...))
		}
		if c.options.datastore, err = datastore.NewClient(ctx, opts...); err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) loadCachestore(ctx context.Context) (err error) {
	if c.options.cachestoreOptions != nil {
		if c.options.cachestore, err = cachestore.NewClient(ctx, c.options.cachestoreOptions...); err != nil {
			return err
		}
	} else {
		// we initialize a memory cache by default, if no other cache connection has been set
		if c.options.cachestore, err = cachestore.NewClient(ctx, cachestore.WithFreeCache()); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) loadTransactionHandler(ctx context.Context) (err error) {
	// the metamorph config will override any other node config set
	if len(c.options.metamorphs) > 0 {
		locationService := NewMetamorphLocationCacheService(ctx, c.Cachestore())
		var metamorph *Metamorph
		metamorph, err = NewMetamorph(c.options.metamorphs, locationService)
		if err != nil {
			return err
		}
		c.options.nodes = []TransactionHandler{metamorph}
	} else if c.options.nodes == nil {
		if len(c.options.nodesOptions) > 0 {
			c.options.nodes = []TransactionHandler{}
			for _, node := range c.options.nodesOptions {
				var bitcoinNode *BitcoinNode
				if bitcoinNode, err = NewBitcoinNode(node.Host, node.Port, node.User, node.Password, node.UseSSL); err != nil {
					return err
				}
				c.options.nodes = append(c.options.nodes, bitcoinNode)
			}
		} else {
			// no node config set, default to localhost
			var bitcoinNode *BitcoinNode
			if bitcoinNode, err = NewBitcoinNode("localhost", 8332, "bitcoin", "bitcoin", false); err != nil {
				return err
			}
			c.options.nodes = []TransactionHandler{bitcoinNode}
		}
	}
	return nil
}

func (c *Client) GetMinerID() (minerID string) {
	if c.options.minerID != nil {
		var err error
		minerID, err = c.options.minerID.GetMinerID()
		if err != nil {
			logger.Fatalf(dictionary.GetInternalMessage(dictionary.ErrorGettingMinerID), err.Error())
		}
	}

	return minerID
}

// Models returns the models registered with the client
func (c *Client) Models() []interface{} {
	return c.options.migrateModels
}

// Datastore returns the datastore being used
func (c *Client) Datastore() datastore.ClientInterface {
	return c.options.datastore
}

func (c *Client) Cachestore() cachestore.ClientInterface {
	return c.options.cachestore
}
