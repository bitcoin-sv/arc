package nats_mq

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/bitcoin-sv/arc/blocktx"
	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

const (
	consumerQueue   = "register-tx-group"
	registerTxTopic = "register-tx"
	minedTxsTopic   = "mined-txs"
	connectionTries = 5
)

type MQClient struct {
	logger       *slog.Logger
	nc           *nats.Conn
	txChannel    chan []byte
	subscription *nats.Subscription
}

func NewNatsMQClient(txChannel chan []byte, logger *slog.Logger, natsURL string) (blocktx.MessageQueueClient, error) {
	var nc *nats.Conn
	var err error

	nc, err = nats.Connect(natsURL)
	if err == nil {
		return &MQClient{nc: nc, logger: logger, txChannel: txChannel}, nil
	}

	ec, err := nats.NewEncodedConn(nc, nats.JSON_ENCODER)
	if err != nil {
		return nil, err
	}
	defer ec.Close()

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

	return &MQClient{nc: nc, logger: logger, txChannel: txChannel}, nil
}

func (c MQClient) SubscribeRegisterTxs() error {

	subscription, err := c.nc.QueueSubscribe(registerTxTopic, consumerQueue, func(msg *nats.Msg) {
		c.txChannel <- msg.Data
	})

	c.subscription = subscription

	if err != nil {
		return err
	}

	return nil
}

func (c MQClient) PublishMinedTxs(txsBlocks *blocktx_api.TransactionBlocks) error {
	data, err := proto.Marshal(txsBlocks)
	if err != nil {
		return err
	}

	err = c.nc.Publish(minedTxsTopic, data)
	if err != nil {
		return err
	}

	return nil
}

func (c MQClient) Shutdown() error {
	err := c.subscription.Unsubscribe()
	if err != nil {
		return err
	}

	c.nc.Close()

	err = c.nc.Drain()
	if err != nil {
		return err
	}

	c.nc.Close()

	return nil
}
