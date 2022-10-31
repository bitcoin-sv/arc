package client

import (
	"context"

	"github.com/mrz1836/go-datastore"
	"github.com/ordishs/go-bitcoin"
)

// TestClient is a Client compatible struct that can be used in tests
type TestClient struct {
	Store datastore.ClientInterface
	Node  Node
}

// Close is a noop
func (t *TestClient) Close() {
	// noop
}

func (t *TestClient) Load(_ context.Context) (err error) {
	return nil
}

func (t *TestClient) Datastore() datastore.ClientInterface {
	return t.Store
}

func (t *TestClient) GetMinerID() (minerID string) {
	return "test miner"
}

func (t *TestClient) GetNode(index int) Node {
	return t.Node
}

func (t *TestClient) GetNodes() []Node {
	return []Node{t.Node}
}

func (t *TestClient) GetRandomNode() Node {
	return t.Node
}

func (t *TestClient) GetRandomNodes(_ int) []Node {
	return []Node{t.Node}
}

func (t *TestClient) Models() []interface{} {
	return nil
}

func (t *TestClient) GetTransactionFromNodes(txID string) (*bitcoin.RawTransaction, error) {
	return t.Node.GetRawTransaction(txID)
}
