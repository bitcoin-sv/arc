package test

import (
	"context"

	"github.com/TAAL-GmbH/mapi/client"
	"github.com/TAAL-GmbH/mapi/config"
	"github.com/TAAL-GmbH/mapi/dictionary"
	"github.com/mrz1836/go-datastore"
	"github.com/mrz1836/go-logger"
)

// Client is a Client compatible struct that can be used in tests
type Client struct {
	Store         datastore.ClientInterface
	Node          client.TransactionHandler
	MinerIDConfig *config.MinerIDConfig
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
	if t.MinerIDConfig != nil {
		var err error
		minerID, err = t.MinerIDConfig.GetMinerID()
		if err != nil {
			logger.Fatalf(dictionary.GetInternalMessage(dictionary.ErrorGettingMinerID), err.Error())
		}
	}

	return minerID
}

func (t *Client) GetNode(_ int) client.TransactionHandler {
	return t.Node
}

func (t *Client) GetNodes() []client.TransactionHandler {
	return []client.TransactionHandler{t.Node}
}

func (t *Client) GetRandomNode() client.TransactionHandler {
	return t.Node
}

func (t *Client) Models() []interface{} {
	return nil
}
