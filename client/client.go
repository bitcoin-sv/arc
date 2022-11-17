package client

import (
	"context"
	"sync"

	"github.com/TAAL-GmbH/mapi/dictionary"
	"github.com/mrz1836/go-datastore"
	"github.com/mrz1836/go-logger"
	"github.com/ordishs/go-bitcoin"
)

type Interface interface {
	Close()
	Load(ctx context.Context) (err error)
	Datastore() datastore.ClientInterface
	GetMinerID() (minerID string)
	GetNode(index int) TransactionHandler
	GetNodes() []TransactionHandler
	GetRandomNode() TransactionHandler
	GetRandomNodes(number int) []TransactionHandler
	Models() []interface{}
	GetTransactionFromNodes(ctx context.Context, txID string) (*bitcoin.RawTransaction, error)
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
	//TODO implement me
	return c.options.nodes[0]
}

func (c *Client) GetRandomNodes(number int) []TransactionHandler {
	//TODO implement me
	return c.options.nodes
}

func (c *Client) GetTransactionFromNodes(ctx context.Context, txID string) (*bitcoin.RawTransaction, error) {

	btTxChan := make(chan *bitcoin.RawTransaction)
	var wg sync.WaitGroup
	for _, node := range c.options.nodes {
		wg.Add(1)
		go func(node TransactionHandler) {
			if tx, err := node.GetTransaction(ctx, txID); err == nil && tx != nil {
				btTxChan <- tx
			}
			wg.Done()
		}(node)
	}

	go func() {
		wg.Wait()
		close(btTxChan)
	}()

	return <-btTxChan, nil
}

// Load all services in the Client
func (c *Client) Load(ctx context.Context) (err error) {

	if c.options.datastoreOptions != nil {
		var opts []datastore.ClientOps
		opts = append(opts, c.options.datastoreOptions)
		if len(c.Models()) > 0 {
			// this will create the base tables from the gorm definition
			opts = append(opts, datastore.WithAutoMigrate(c.Models()...))
		}
		c.options.datastore, err = datastore.NewClient(ctx, opts...)
	} else if c.options.datastore == nil {
		// we initialize a SQL lite db by default, if no other database connection has been set
		var opts []datastore.ClientOps
		opts = append(opts, datastore.WithSQLite(&datastore.SQLiteConfig{
			DatabasePath: "./mapi.db",
			Shared:       true,
		}))
		if len(c.Models()) > 0 {
			// this will create the base tables from the gorm definition
			opts = append(opts, datastore.WithAutoMigrate(c.Models()...))
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

// Models returns the models registered with the client
func (c *Client) Models() []interface{} {
	return c.options.migrateModels
}

// Datastore returns the datastore being used
func (c *Client) Datastore() datastore.ClientInterface {

	return c.options.datastore
}
