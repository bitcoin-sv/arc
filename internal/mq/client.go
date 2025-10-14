package mq

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"

	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/pkg/message_queue/nats/client/nats_jetstream"
	"github.com/bitcoin-sv/arc/pkg/message_queue/nats/nats_connection"
)

const (
	SubmitTxTopic    = "submit-tx"
	MinedTxsTopic    = "mined-txs"
	RegisterTxTopic  = "register-tx"
	RegisterTxsTopic = "register-txs"
	CallbackTopic    = "callback"
)

type MessageQueueClient interface {
	// PublishCore publishes a message as byte array to the specified topic
	PublishCore(topic string, data []byte) (err error)
	// PublishMarshalCore publishes a message as proto message to the specified topic
	PublishMarshalCore(topic string, m proto.Message) (err error)

	// Publish publishes a message as byte array to a topic persisted in a stream
	Publish(ctx context.Context, topic string, data []byte) error
	// PublishAsync publishes a message as byte array to the specified topic persisted in a stream not blocking while waiting for an acknowledgement
	PublishAsync(topic string, hash []byte) (err error)
	// PublishMarshal publishes a message as proto message to a topic persisted in a stream
	PublishMarshal(ctx context.Context, topic string, m proto.Message) error
	// PublishMarshalAsync publishes a message as proto message to the specified topic persisted in a stream not blocking while waiting for an acknowledgement
	PublishMarshalAsync(topic string, m proto.Message) error

	// Consume subscribes to a topic to consume a stream and calls the specified function for each message as byte array
	Consume(topic string, msgFunc func([]byte) error) error
	// ConsumeMsg subscribes to a topic to consume a stream and calls the specified function for each message as jetstream.Msg
	ConsumeMsg(topic string, msgFunc func(msg jetstream.Msg) error) error
	// QueueSubscribe subscribes to a topic and calls the specified function for each message as byte array
	QueueSubscribe(topic string, msgFunc func([]byte) error) error

	Status() nats.Status
	IsConnected() bool
	Shutdown()
}

func NewMqClient(logger *slog.Logger, mqCfg *config.MessageQueueConfig, jsOpts ...nats_jetstream.Option) (MessageQueueClient, error) {
	if mqCfg == nil {
		return nil, errors.New("mqCfg is required")
	}

	logger = logger.With("module", "message-queue")

	var conn *nats.Conn
	var err error

	connOpts := []nats_connection.Option{
		nats_connection.WithMaxReconnects(-1),
		nats_connection.WithUser(mqCfg.User),
		nats_connection.WithPassword(mqCfg.Password),
	}

	conn, err = nats_connection.New(mqCfg.URL, logger, connOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to establish connection to message queue at URL %s: %v", mqCfg.URL, err)
	}
	if !mqCfg.Streaming.Enabled {
		return nil, errors.New("currently only message queue with streaming supported")
	}
	if mqCfg.Streaming.FileStorage {
		jsOpts = append(jsOpts, nats_jetstream.WithFileStorage())
	}

	var mqClient *nats_jetstream.Client
	mqClient, err = nats_jetstream.New(conn, logger, jsOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create nats client: %v", err)
	}

	return mqClient, nil
}
