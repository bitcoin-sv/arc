package nats_mq

import (
	"context"
	"crypto/rand"
	"log/slog"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-nats/pkg/nats"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/nats-io/stan.go"
	"github.com/oklog/ulid"
)

type Publisher struct {
	publisher *nats.StreamingPublisher
	topic     string
	logger    *slog.Logger
}

func NewNatsMQPublisher(logger *slog.Logger) (*Publisher, error) {

	const topic = "register-tx"
	const clusterID = "test-cluster"
	const natsURL = "nats://nats:4222"

	publisher, err := nats.NewStreamingPublisher(
		nats.StreamingPublisherConfig{
			ClusterID: clusterID,
			ClientID:  "publisher",
			StanOptions: []stan.Option{
				stan.NatsURL(natsURL),
			},
			Marshaler: nats.GobMarshaler{},
		},
		watermill.NewStdLogger(false, false),
	)
	if err != nil {
		return nil, err
	}

	return &Publisher{publisher: publisher, topic: topic, logger: logger}, nil
}

func (p Publisher) PublishTransaction(ctx context.Context, hash []byte) error {
	uuid := ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader)
	msg := message.NewMessage(uuid.String(), hash)

	txHash, err := chainhash.NewHash(msg.Payload)
	if err != nil {
		p.logger.Error("failed to parse payload", slog.String("err", err.Error()))
		return err
	}
	p.logger.Info("SENDING TX OVER NATS", slog.String("hash", txHash.String()))

	err = p.publisher.Publish(p.topic, msg)
	if err != nil {
		return err
	}

	return nil
}
