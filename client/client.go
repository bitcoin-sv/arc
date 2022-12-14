package client

import (
	"context"

	"github.com/TAAL-GmbH/arc/api"
	"github.com/mrz1836/go-cachestore"
)

type Interface interface {
	Load(ctx context.Context) (err error)
	Cachestore() cachestore.ClientInterface
	GetDefaultFees() []api.Fee
	GetTransactionHandler() TransactionHandler
}

func New(opts ...Options) (Interface, error) {
	client := &Client{
		&clientOptions{},
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

// GetTransactionHandler returns the loaded transaction handler
func (c *Client) GetTransactionHandler() TransactionHandler {
	return c.options.transactionHandler
}

// GetDefaultFees returns the default fees from the config
func (c *Client) GetDefaultFees() []api.Fee {
	return c.options.fees
}

// Load all services in the Client
func (c *Client) Load(ctx context.Context) (err error) {

	if err = c.loadCachestore(ctx); err != nil {
		return err
	}

	if err = c.loadTransactionHandler(ctx); err != nil {
		return err
	}

	return
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
		c.options.transactionHandler = metamorph
	} else if c.options.transactionHandler == nil {
		if len(c.options.nodesOptions) > 0 {
			node := c.options.nodesOptions[0]
			var bitcoinNode *BitcoinNode
			if bitcoinNode, err = NewBitcoinNode(node.Host, node.Port, node.User, node.Password, node.UseSSL); err != nil {
				return err
			}
			c.options.transactionHandler = bitcoinNode
		} else {
			// no node config set, default to localhost
			var bitcoinNode *BitcoinNode
			if bitcoinNode, err = NewBitcoinNode("localhost", 8332, "bitcoin", "bitcoin", false); err != nil {
				return err
			}
			c.options.transactionHandler = bitcoinNode
		}
	}
	return nil
}

func (c *Client) Cachestore() cachestore.ClientInterface {
	return c.options.cachestore
}
