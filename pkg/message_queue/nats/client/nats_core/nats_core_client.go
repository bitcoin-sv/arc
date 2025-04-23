package nats_core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/protobuf/proto"

	"github.com/bitcoin-sv/arc/pkg/tracing"
)

var (
	ErrFailedToPublish   = errors.New("failed to publish")
	ErrFailedToSubscribe = errors.New("failed to subscribe")
)

type NatsConnection interface {
	QueueSubscribe(subj, queue string, cb nats.MsgHandler) (*nats.Subscription, error)
	Publish(subj string, data []byte) error
	Status() nats.Status
	Drain() error
}

func WithTracer(attr ...attribute.KeyValue) func(p *Client) {
	return func(p *Client) {
		p.tracingEnabled = true
		if len(attr) > 0 {
			p.tracingAttributes = append(p.tracingAttributes, attr...)
		}
		_, file, _, ok := runtime.Caller(1)
		if ok {
			p.tracingAttributes = append(p.tracingAttributes, attribute.String("file", file))
		}
	}
}

type Client struct {
	nc     NatsConnection
	logger *slog.Logger

	tracingEnabled    bool
	tracingAttributes []attribute.KeyValue
}

func WithLogger(logger *slog.Logger) func(p *Client) {
	return func(m *Client) {
		m.logger = logger
	}
}

type Option func(p *Client)

func New(nc NatsConnection, opts ...Option) *Client {
	m := &Client{
		nc:     nc,
		logger: slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})),
	}
	for _, opt := range opts {
		opt(m)
	}

	return m
}

func (c Client) Status() nats.Status {
	return c.nc.Status()
}

func (c Client) Shutdown() {
	if c.nc != nil {
		err := c.nc.Drain()
		if err != nil {
			c.logger.Error("failed to drain nats connection", slog.String("err", err.Error()))
		}
	}
}

func (c Client) Publish(ctx context.Context, topic string, data []byte) (err error) {
	_, span := tracing.StartTracing(ctx, "Publish", c.tracingEnabled, c.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	err = c.nc.Publish(topic, data)
	if err != nil {
		return errors.Join(ErrFailedToPublish, fmt.Errorf(topic, topic), err)
	}

	return nil
}

func (c Client) PublishMarshal(ctx context.Context, topic string, m proto.Message) (err error) {
	ctx, span := tracing.StartTracing(ctx, "PublishMarshal", c.tracingEnabled, c.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	data, err := proto.Marshal(m)
	if err != nil {
		return err
	}

	err = c.Publish(ctx, topic, data)
	if err != nil {
		return errors.Join(ErrFailedToPublish, fmt.Errorf(topic, topic), err)
	}

	return nil
}

func (c Client) Subscribe(topic string, msgFunc func([]byte) error) error {
	_, err := c.nc.QueueSubscribe(topic, topic+"-group", func(msg *nats.Msg) {
		err := msgFunc(msg.Data)
		if err != nil {
			c.logger.Error(fmt.Sprintf("failed to run message function on %s topic", topic), slog.String("err", err.Error()))
		}
	})
	if err != nil {
		return errors.Join(ErrFailedToSubscribe, fmt.Errorf(topic, topic), err)
	}

	return nil
}
