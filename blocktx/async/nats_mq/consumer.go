package nats_mq

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	consumerQueue   = "register-tx-group"
	topic           = "register-tx"
	connectionTries = 5
)

type Consumer struct {
	logger       *slog.Logger
	nc           *nats.Conn
	topic        string
	txChannel    chan []byte
	subscription *nats.Subscription
}

func NewNatsMQConsumer(txChannel chan []byte, logger *slog.Logger, natsURL string) (*Consumer, error) {
	var nc *nats.Conn
	var err error

	nc, err = nats.Connect(natsURL)
	if err == nil {
		return &Consumer{nc: nc, logger: logger, topic: topic, txChannel: txChannel}, nil
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

	return &Consumer{nc: nc, logger: logger, topic: topic, txChannel: txChannel}, nil
}

func (c Consumer) ConsumeTransactions() error {

	subscription, err := c.nc.QueueSubscribe(c.topic, consumerQueue, func(msg *nats.Msg) {
		c.txChannel <- msg.Data
	})

	c.subscription = subscription

	if err != nil {
		return err
	}

	return nil
}

func (c Consumer) Shutdown() error {
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
