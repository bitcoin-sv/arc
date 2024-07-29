package async

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

const (
	SubmitTxTopic   = "submit-tx"
	MinedTxsTopic   = "mined-txs"
	RegisterTxTopic = "register-tx"
	RequestTxTopic  = "request-tx"
)

type NatsClient interface {
	QueueSubscribe(subj, queue string, cb nats.MsgHandler) (*nats.Subscription, error)
	Close()
	Publish(subj string, data []byte) error
	Drain() error
}

type MQClient struct {
	nc     NatsClient
	logger *slog.Logger
}

func WithLogger(logger *slog.Logger) func(handler *MQClient) {
	return func(m *MQClient) {
		m.logger = logger
	}
}

func NewNatsMQClient(nc NatsClient, opts ...func(client *MQClient)) *MQClient {
	m := &MQClient{
		nc:     nc,
		logger: slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})),
	}
	for _, opt := range opts {
		opt(m)
	}

	return m
}

func (c MQClient) Shutdown() {
	if c.nc != nil {
		err := c.nc.Drain()
		if err != nil {
			c.logger.Error("failed to drain nats connection", slog.String("err", err.Error()))
		}
	}
}

func (c MQClient) Publish(topic string, data []byte) error {
	err := c.nc.Publish(topic, data)
	if err != nil {
		return fmt.Errorf("failed to publish on %s topic: %w", topic, err)
	}

	return nil
}

func (c MQClient) PublishMarshal(topic string, m proto.Message) error {
	data, err := proto.Marshal(m)
	if err != nil {
		return err
	}

	err = c.Publish(topic, data)
	if err != nil {
		return fmt.Errorf("failed to publish on %s topic: %w", topic, err)
	}

	return nil
}

func (c MQClient) Subscribe(topic string, msgFunc func([]byte) error) error {

	_, err := c.nc.QueueSubscribe(topic, topic+"-group", func(msg *nats.Msg) {
		err := msgFunc(msg.Data)
		if err != nil {
			c.logger.Error(fmt.Sprintf("failed to subscribe on %s topic", topic))
		}
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to %s topic: %w", topic, err)
	}

	return nil
}
