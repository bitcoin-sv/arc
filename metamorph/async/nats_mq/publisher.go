package nats_mq

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	topic           = "register-tx"
	connectionTries = 5
)

type Publisher struct {
	topic  string
	logger *slog.Logger
	nc     *nats.Conn
}

func NewNatsMQPublisher(logger *slog.Logger, natsURL string) (*Publisher, error) {

	var nc *nats.Conn
	var err error

	nc, err = nats.Connect(natsURL)
	if err == nil {
		return &Publisher{nc: nc, logger: logger, topic: topic}, nil
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

	return &Publisher{nc: nc, topic: topic, logger: logger}, nil
}

func (p Publisher) PublishTransaction(hash []byte) error {
	err := p.nc.Publish(p.topic, hash)
	if err != nil {
		return err
	}

	return nil
}

func (p Publisher) Shutdown() error {

	err := p.nc.Drain()
	if err != nil {
		return err
	}

	p.nc.Close()

	return nil
}
