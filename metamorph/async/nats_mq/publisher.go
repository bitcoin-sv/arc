package nats_mq

import (
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
)

const topic = "register-tx"

type Publisher struct {
	topic  string
	logger *slog.Logger
	nc     *nats.Conn
}

func NewNatsMQPublisher(logger *slog.Logger, natsURL string) (*Publisher, error) {

	var nc *nats.Conn
	var err error
	for i := 0; i < 5; i++ {
		nc, err = nats.Connect(natsURL)
		if err == nil {
			break
		}

		logger.Info("Waiting before connecting to NATS", slog.String("url", natsURL))
		time.Sleep(1 * time.Second)
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
