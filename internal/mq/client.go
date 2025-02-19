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
	SubmitTxTopic   = "submit-tx"
	MinedTxsTopic   = "mined-txs"
	RegisterTxTopic = "register-tx"
	CallbackTopic   = "callback"
)

type MessageQueueClient interface {
	Publish(ctx context.Context, topic string, data []byte) error
	PublishMarshal(ctx context.Context, topic string, m proto.Message) error
	Subscribe(topic string, msgFunc func([]byte) error) error
	SubscribeMsg(topic string, msgFunc func(msg jetstream.Msg) error) error
	Shutdown()
}

func NewMqClient(ctx context.Context, logger *slog.Logger, arcConfig *config.ArcConfig, opts ...nats_jetstream.Option) (MessageQueueClient, error) {
	logger = logger.With("module", "nats")

	clientClosedCh := make(chan struct{}, 1)

	var conn *nats.Conn
	var err error

	conn, err = nats_connection.New(arcConfig.MessageQueue.URL, logger, nats_connection.WithClientClosedChannel(clientClosedCh))
	if err != nil {
		return nil, fmt.Errorf("failed to establish connection to message queue at URL %s: %v", arcConfig.MessageQueue.URL, err)
	}

	// recreate connection if it closes
	go func() {
		for {
			select {
			case <-clientClosedCh:
				// Todo: create connection
				//conn, err = nats_connection.New(arcConfig.MessageQueue.URL, logger, nats_connection.WithClientClosedChannel(clientClosedCh))
				//if err != nil {
				//	logger.Error("Failed to create connection to message queue at URL %s: %v", arcConfig.MessageQueue.URL, err)
				//}
			case <-ctx.Done():
				return
			}
		}
	}()

	var mqClient MessageQueueClient
	if !arcConfig.MessageQueue.Streaming.Enabled {
		return nil, errors.New("currently only message queue with streaming supported")
	}
	if arcConfig.MessageQueue.Streaming.FileStorage {
		opts = append(opts, nats_jetstream.WithFileStorage())
	}

	if arcConfig.Tracing.Enabled {
		opts = append(opts, nats_jetstream.WithTracer(arcConfig.Tracing.KeyValueAttributes...))
	}

	mqClient, err = nats_jetstream.New(conn, logger, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create nats client: %v", err)
	}

	return mqClient, nil
}
