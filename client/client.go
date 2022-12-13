package client

import (
	"context"

	"github.com/TAAL-GmbH/arc"
	"github.com/mrz1836/go-cachestore"
)

type Interface interface {
	Load(ctx context.Context) (err error)
	Cachestore() cachestore.ClientInterface
	GetDefaultFees() []arc.Fee
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
func (c *Client) GetDefaultFees() []arc.Fee {
	return c.options.fees
}

// Load all services in the Client
func (c *Client) Load(ctx context.Context) (err error) {

	if err = c.loadCachestore(ctx); err != nil {
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

func (c *Client) Cachestore() cachestore.ClientInterface {
	return c.options.cachestore
}
