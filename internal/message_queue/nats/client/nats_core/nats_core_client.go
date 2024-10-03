package natscore

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

var (
	ErrFailedToPublish   = fmt.Errorf("failed to publish")
	ErrFailedToSubscribe = fmt.Errorf("failed to subscribe")
)

type NatsConnection interface {
	QueueSubscribe(subj, queue string, cb nats.MsgHandler) (*nats.Subscription, error)
	Publish(subj string, data []byte) error
	Drain() error
}

type Client struct {
	nc     NatsConnection
	logger *slog.Logger
}

func WithLogger(logger *slog.Logger) func(handler *Client) {
	return func(m *Client) {
		m.logger = logger
	}
}

func New(nc NatsConnection, opts ...func(client *Client)) *Client {
	m := &Client{
		nc:     nc,
		logger: slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})),
	}
	for _, opt := range opts {
		opt(m)
	}

	return m
}

func (c Client) Shutdown() {
	if c.nc != nil {
		err := c.nc.Drain()
		if err != nil {
			c.logger.Error("failed to drain nats connection", slog.String("err", err.Error()))
		}
	}
}

func (c Client) Publish(topic string, data []byte) error {
	err := c.nc.Publish(topic, data)
	if err != nil {
		return errors.Join(ErrFailedToPublish, fmt.Errorf("topic: %s", topic), err)
	}

	return nil
}

func (c Client) PublishMarshal(topic string, m proto.Message) error {
	data, err := proto.Marshal(m)
	if err != nil {
		return err
	}

	err = c.Publish(topic, data)
	if err != nil {
		return errors.Join(ErrFailedToPublish, fmt.Errorf("topic: %s", topic), err)
	}

	return nil
}

func (c Client) Subscribe(topic string, msgFunc func([]byte) error) error {

	_, err := c.nc.QueueSubscribe(topic, topic+"-group", func(msg *nats.Msg) {
		err := msgFunc(msg.Data)
		if err != nil {
			c.logger.Error(fmt.Sprintf("failed to run message function on %s topic", topic))
		}
	})
	if err != nil {
		return errors.Join(ErrFailedToSubscribe, fmt.Errorf("topic: %s", topic), err)
	}

	return nil
}
