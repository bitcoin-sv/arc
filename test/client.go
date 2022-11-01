package test

import (
	"context"

	"github.com/TAAL-GmbH/mapi/client"
	"github.com/mrz1836/go-datastore"
	"github.com/ordishs/go-bitcoin"
)

// Client is a Client compatible struct that can be used in tests
type Client struct {
	Store datastore.ClientInterface
	Node  client.Node
}

// Close is a noop
func (t *Client) Close() {
	// noop
}

func (t *Client) Load(_ context.Context) (err error) {
	return nil
}

func (t *Client) Datastore() datastore.ClientInterface {
	return t.Store
}

func (t *Client) GetMinerID() (minerID string) {
	return "test miner"
}

func (t *Client) GetNode(index int) client.Node {
	return t.Node
}

func (t *Client) GetNodes() []client.Node {
	return []client.Node{t.Node}
}

func (t *Client) GetRandomNode() client.Node {
	return t.Node
}

func (t *Client) GetRandomNodes(_ int) []client.Node {
	return []client.Node{t.Node}
}

func (t *Client) Models() []interface{} {
	return nil
}

func (t *Client) GetTransactionFromNodes(txID string) (*bitcoin.RawTransaction, error) {
	return t.Node.GetRawTransaction(txID)
}
