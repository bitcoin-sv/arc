package nats_mq

import (
	"context"
	"log/slog"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-nats/pkg/nats"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/message/router/middleware"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/nats-io/stan.go"
)

type Consumer struct {
	router *message.Router
	logger *slog.Logger
}

func NewNatsMQConsumer(txChannel chan []byte, logger *slog.Logger) (*Consumer, error) {

	watermillLogger := watermill.NewStdLogger(false, false)

	const topic = "register-tx"
	const clusterID = "test-cluster"
	const natsURL = "nats://nats:4222"

	subscriber, err := nats.NewStreamingSubscriber(
		nats.StreamingSubscriberConfig{
			ClusterID:   clusterID,
			ClientID:    "subscriber",
			QueueGroup:  "example-group",
			DurableName: "example-durable",
			StanOptions: []stan.Option{
				stan.NatsURL(natsURL),
			},
			Unmarshaler: nats.GobMarshaler{},
		},
		watermillLogger,
	)
	if err != nil {
		return nil, err
	}

	router, err := message.NewRouter(message.RouterConfig{}, watermillLogger)
	if err != nil {
		return nil, err
	}
	router.AddMiddleware(middleware.Recoverer)

	router.AddNoPublisherHandler(
		"messages_handler",
		topic,
		subscriber,
		func(msg *message.Message) error {

			hash, err := chainhash.NewHash(msg.Payload)
			if err != nil {
				logger.Error("failed to parse payload", slog.String("err", err.Error()))
				return err
			}
			logger.Info("RECEIVED TX OVER NATS", slog.String("hash", hash.String()))

			txChannel <- msg.Payload
			return nil
		},
	)

	return &Consumer{router: router, logger: logger}, nil
}

func (c Consumer) ConsumeTransactions(ctx context.Context) error {

	go func() {
		err := c.router.Run(context.Background())
		if err != nil {
			c.logger.Error("failed to run router", slog.String("err", err.Error()))
		}
	}()
	return nil
}

func (c Consumer) Shutdown() error {
	return c.router.Close()
}
