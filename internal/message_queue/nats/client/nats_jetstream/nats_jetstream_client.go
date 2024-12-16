package nats_jetstream

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/protobuf/proto"

	"github.com/bitcoin-sv/arc/internal/tracing"
)

type Client struct {
	js          jetstream.JetStream
	nc          *nats.Conn
	logger      *slog.Logger
	consumers   map[string]jetstream.Consumer
	storageType jetstream.StorageType
	ctx         context.Context
	cancelAll   context.CancelFunc

	tracingEnabled    bool
	tracingAttributes []attribute.KeyValue
}

var (
	ErrConsumerNotInitialized = errors.New("consumer for topic not initialized")
	ErrFailedToGetStream      = errors.New("failed to get stream")
	ErrFailedToGetConsumer    = errors.New("failed to get consumer")
	ErrFailedToApplyOption    = errors.New("failed to apply option")
	ErrFailedToCreateStream   = errors.New("failed to create stream")
	ErrFailedToCreateConsumer = errors.New("failed to create consumer")
	ErrFailedToPublish        = errors.New("failed to publish")
	ErrFailedToSubscribe      = errors.New("failed to subscribe")
)

func WithSubscribedTopics(topics ...string) func(handler *Client) error {
	return func(c *Client) error {
		for _, topic := range topics {
			streamMinedTxs, err := c.getStream(topic, fmt.Sprintf("%s-stream", topic))
			if err != nil {
				return errors.Join(ErrFailedToGetStream, err)
			}

			cons, err := c.getConsumer(streamMinedTxs, fmt.Sprintf("%s-cons", topic))
			if err != nil {
				return errors.Join(ErrFailedToGetConsumer, err)
			}

			c.consumers[topic] = cons
		}

		return nil
	}
}

func WithFileStorage() func(handler *Client) error {
	return func(c *Client) error {
		c.storageType = jetstream.FileStorage
		return nil
	}
}
func WithTracer(attr ...attribute.KeyValue) func(*Client) error {
	return func(p *Client) error {
		p.tracingEnabled = true
		if len(attr) > 0 {
			p.tracingAttributes = append(p.tracingAttributes, attr...)
		}
		_, file, _, ok := runtime.Caller(1)
		if ok {
			p.tracingAttributes = append(p.tracingAttributes, attribute.String("file", file))
		}

		return nil
	}
}

type Option func(p *Client) error

func New(nc *nats.Conn, logger *slog.Logger, topics []string, opts ...Option) (*Client, error) {
	ctx, cancel := context.WithCancel(context.Background())

	p := &Client{
		logger:      logger.With("module", "nats-jetstream"),
		nc:          nc,
		consumers:   map[string]jetstream.Consumer{},
		storageType: jetstream.MemoryStorage,
		ctx:         ctx,
		cancelAll:   cancel,
	}

	js, err := jetstream.New(nc)
	if err != nil {
		return nil, err
	}

	p.js = js

	for _, topic := range topics {
		_, err = p.getStream(topic, fmt.Sprintf("%s-stream", topic))
		if err != nil {
			return nil, errors.Join(ErrFailedToGetStream, err)
		}
	}

	for _, opt := range opts {
		err = opt(p)
		if err != nil {
			return nil, errors.Join(ErrFailedToApplyOption, err)
		}
	}
	return p, nil
}

func (cl *Client) getStream(topicName string, streamName string) (jetstream.Stream, error) {
	streamCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	stream, err := cl.js.Stream(streamCtx, streamName)
	if errors.Is(err, jetstream.ErrStreamNotFound) {
		cl.logger.Info(fmt.Sprintf("stream %s not found, creating new", streamName))

		stream, err = cl.js.CreateStream(streamCtx, jetstream.StreamConfig{
			Name:        streamName,
			Description: "Stream for topic " + topicName,
			Subjects:    []string{topicName},
			Retention:   jetstream.WorkQueuePolicy,
			Discard:     jetstream.DiscardOld,
			MaxAge:      10 * time.Minute,
			Storage:     cl.storageType,
			NoAck:       false,
		})
		if err != nil {
			return nil, errors.Join(ErrFailedToCreateStream, err)
		}

		cl.logger.Info(fmt.Sprintf("stream %s created", streamName))
		return stream, nil
	} else if err != nil {
		return nil, err
	}

	cl.logger.Info(fmt.Sprintf("stream %s found", streamName))
	return stream, nil
}

func (cl *Client) getConsumer(stream jetstream.Stream, consumerName string) (jetstream.Consumer, error) {
	consCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cons, err := stream.Consumer(consCtx, consumerName)
	if errors.Is(err, jetstream.ErrConsumerNotFound) {
		cl.logger.Warn(fmt.Sprintf("consumer %s not found, creating new", consumerName))
		cons, err = stream.CreateConsumer(consCtx, jetstream.ConsumerConfig{
			Durable:   consumerName,
			AckPolicy: jetstream.AckExplicitPolicy,
		})
		if err != nil {
			return nil, errors.Join(ErrFailedToCreateConsumer, err)
		}

		cl.logger.Info(fmt.Sprintf("consumer %s created", consumerName))
		return cons, nil
	} else if err != nil {
		return nil, err
	}

	cl.logger.Info(fmt.Sprintf("consumer %s found", consumerName))
	return cons, nil
}

func (cl *Client) Publish(ctx context.Context, topic string, hash []byte) (err error) {
	ctx, span := tracing.StartTracing(ctx, "Publish", cl.tracingEnabled, cl.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	_, err = cl.js.Publish(ctx, topic, hash)
	if err != nil {
		return errors.Join(ErrFailedToPublish, fmt.Errorf("topic: %s", topic), err)
	}

	return nil
}

func (cl *Client) PublishMarshal(ctx context.Context, topic string, m proto.Message) (err error) {
	ctx, span := tracing.StartTracing(ctx, "PublishMarshal", cl.tracingEnabled, cl.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	data, err := proto.Marshal(m)
	if err != nil {
		return err
	}

	err = cl.Publish(ctx, topic, data)
	if err != nil {
		return errors.Join(ErrFailedToPublish, fmt.Errorf("topic: %s", topic), err)
	}

	return nil
}

func (cl *Client) Subscribe(topic string, msgFunc func([]byte) error) error {
	consumer, found := cl.consumers[topic]

	if !found {
		return ErrConsumerNotInitialized
	}

	_, err := consumer.Consume(func(msg jetstream.Msg) {
		msgErr := msgFunc(msg.Data())
		if msgErr != nil {
			cl.logger.Error(fmt.Sprintf("failed to consume message on %s topic: %s", topic, string(msg.Data())), slog.String("err", msgErr.Error()))
			return
		}

		ackErr := msg.Ack()
		if ackErr != nil {
			cl.logger.Error(fmt.Sprintf("failed to acknowledge message on %s topic: %s", topic, string(msg.Data())))
		}
	})
	if err != nil {
		return errors.Join(ErrFailedToSubscribe, fmt.Errorf("topic: %s", topic), err)
	}

	return nil
}

func (cl *Client) Shutdown() {
	if cl.nc != nil {
		err := cl.nc.Drain()
		if err != nil {
			cl.logger.Error("failed to drain nats connection", slog.String("err", err.Error()))
		}
	}

	cl.cancelAll()
}
