package nats_mq

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

const (
	registerTxTopic = "register-tx"

	consumerQueue   = "mined-txs-group"
	minedTxsTopic   = "mined-txs"
	connectionTries = 5
)

type MQClient struct {
	logger       *slog.Logger
	nc           *nats.Conn
	subscription *nats.Subscription
	minedTxsChan chan *blocktx_api.TransactionBlocks
}

func NewNatsMQClient(minedTxsChan chan *blocktx_api.TransactionBlocks, logger *slog.Logger, natsURL string) (metamorph.MessageQueueClient, error) {

	var nc *nats.Conn
	var err error

	nc, err = nats.Connect(natsURL)
	if err == nil {
		return &MQClient{nc: nc, logger: logger, minedTxsChan: minedTxsChan}, nil
	}

	// Try to reconnect in intervals
	i := 0
	for range time.NewTicker(2 * time.Second).C {
		nc, err = nats.Connect(natsURL)
		if err != nil && i >= connectionTries {
			return nil, fmt.Errorf("failed to connect to NATS server: %v", err)
		}

		if err == nil {
			break
		}

		logger.Info("Waiting before connecting to NATS", slog.String("url", natsURL))
		i++
	}

	logger.Info("Connected to NATS at", slog.String("url", nc.ConnectedUrl()))

	return &MQClient{nc: nc, logger: logger, minedTxsChan: minedTxsChan}, nil
}

func (c MQClient) PublishRegisterTxs(hash []byte) error {
	err := c.nc.Publish(registerTxTopic, hash)
	if err != nil {
		return err
	}

	return nil
}

func (c MQClient) SubscribeMinedTxs() error {

	subscription, err := c.nc.QueueSubscribe(minedTxsTopic, consumerQueue, func(msg *nats.Msg) {

		serialized := &blocktx_api.TransactionBlocks{}
		err := proto.Unmarshal(msg.Data, serialized)
		if err != nil {
			c.logger.Error("failed to unmarshal message", slog.String("err", err.Error()))
			return
		}

		c.minedTxsChan <- serialized
	})

	c.subscription = subscription

	if err != nil {
		return err
	}

	return nil
}

func (c MQClient) Shutdown() error {

	err := c.nc.Drain()
	if err != nil {
		return err
	}

	c.nc.Close()

	return nil
}
