package client

import (
	"context"

	"github.com/mrz1836/go-datastore"
	"github.com/mrz1836/go-logger"
	"github.com/taal/mapi/bitcoin"
	"github.com/taal/mapi/dictionary"
)

type Interface interface {
	Close()
	Load(ctx context.Context) (err error)
	Datastore() datastore.ClientInterface
	GetMinerID() (minerID string)
	GetNode(index int) *bitcoin.Node
	GetNodes() []*bitcoin.Node
	GetRandomNode() *bitcoin.Node
	GetRandomNodes(number int) []*bitcoin.Node
}

func New(opts ...Options) (Interface, error) {

	// default to localhost node with rest service
	node := bitcoin.NewRest("http://locahost:8332")
	client := &Client{
		&clientOptions{
			nodes: []*bitcoin.Node{
				&node,
			},
		},
	}

	// Overwrite defaults with any custom options provided by the user
	for _, opt := range opts {
		opt(client.options)
	}

	if err := client.Load(context.Background()); err != nil {
		return nil, err
	}

	return client, nil
}

func (c *Client) Close() {
	_ = c.Datastore().Close(context.Background())
}

func (c *Client) GetNode(index int) *bitcoin.Node {
	if len(c.options.nodes) < index-1 {
		return nil
	}

	return c.options.nodes[index]
}

func (c *Client) GetNodes() []*bitcoin.Node {
	return c.options.nodes
}

func (c *Client) GetRandomNode() *bitcoin.Node {
	//TODO implement me
	return c.options.nodes[0]
}

func (c *Client) GetRandomNodes(number int) []*bitcoin.Node {
	//TODO implement me
	return c.options.nodes
}

// Load all services in the Client
func (c *Client) Load(ctx context.Context) (err error) {

	if c.options.datastoreOptions != nil {
		var opts []datastore.ClientOps
		opts = append(opts, c.options.datastoreOptions)
		if len(c.options.migrateModels) > 0 {
			opts = append(opts, datastore.WithAutoMigrate(c.options.migrateModels...))
		}
		c.options.datastore, err = datastore.NewClient(ctx, opts...)
	} else {
		// we initialize a SQL lite db by default
		var opts []datastore.ClientOps
		opts = append(opts, datastore.WithSQLite(&datastore.SQLiteConfig{
			DatabasePath: "./mapi.db",
			Shared:       true,
		}))
		if len(c.options.migrateModels) > 0 {
			opts = append(opts, datastore.WithAutoMigrate(c.options.migrateModels...))
		}
		c.options.datastore, err = datastore.NewClient(ctx, opts...)
	}

	return
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

// Datastore returns the datastore being used
func (c *Client) Datastore() datastore.ClientInterface {

	return c.options.datastore
}
