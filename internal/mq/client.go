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

func NewMqClient(logger *slog.Logger, mqCfg *config.MessageQueueConfig, tracingCfg *config.TracingConfig, jsOpts []nats_jetstream.Option, connOpts []nats_connection.Option) (MessageQueueClient, error) {
	if mqCfg == nil {
		return nil, errors.New("mqCfg is required")
	}

	logger = logger.With("module", "message-queue")

	clientClosedCh := make(chan struct{}, 1)

	var conn *nats.Conn
	var err error

	connOpts = append(connOpts, nats_connection.WithClientClosedChannel(clientClosedCh))
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

	if tracingCfg != nil && tracingCfg.Enabled {
		jsOpts = append(jsOpts, nats_jetstream.WithTracer(tracingCfg.KeyValueAttributes...))
	}

	var mqClient *nats_jetstream.Client
	mqClient, err = nats_jetstream.New(conn, logger, jsOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create nats client: %v", err)
	}

	return mqClient, nil
}
