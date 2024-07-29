package message_queue

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

type NatsCoreClient struct {
	nc     NatsConnection
	logger *slog.Logger
}

func WithLogger(logger *slog.Logger) func(handler *NatsCoreClient) {
	return func(m *NatsCoreClient) {
		m.logger = logger
	}
}

func NewNatsCoreClient(nc NatsConnection, opts ...func(client *NatsCoreClient)) *NatsCoreClient {
	m := &NatsCoreClient{
		nc:     nc,
		logger: slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})),
	}
	for _, opt := range opts {
		opt(m)
	}

	return m
}

func (c NatsCoreClient) Shutdown() {
	if c.nc != nil {
		err := c.nc.Drain()
		if err != nil {
			c.logger.Error("failed to drain nats connection", slog.String("err", err.Error()))
		}
	}
}

func (c NatsCoreClient) Publish(topic string, data []byte) error {
	err := c.nc.Publish(topic, data)
	if err != nil {
		return fmt.Errorf("failed to publish on %s topic: %w", topic, err)
	}

	return nil
}

func (c NatsCoreClient) PublishMarshal(topic string, m proto.Message) error {
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

func (c NatsCoreClient) Subscribe(topic string, msgFunc func([]byte) error) error {

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
