package async

import (
	"context"
	"errors"
	"fmt"
	"github.com/nats-io/nats.go"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"
)

type Client struct {
	url         string
	js          jetstream.JetStream
	nc          NatsClient
	stream      jetstream.Stream
	logger      *slog.Logger
	consumer    jetstream.Consumer
	ctx         context.Context
	hasConsumer bool
}

func WithConsumer() func(handler *Client) {
	return func(m *Client) {
		m.hasConsumer = true
	}
}

func NewJetStreamClient(ctx context.Context, nc *nats.Conn, logger *slog.Logger, url string, opts ...func(client *Client)) (*Client, error) {

	p := &Client{
		logger: logger,
		url:    url,
		ctx:    ctx,
		nc:     nc,
	}

	for _, opt := range opts {
		opt(p)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		return nil, err
	}

	p.js = js

	stream, err := p.getStream()
	if err != nil {
		return nil, err
	}

	p.stream = stream

	if p.hasConsumer {
		cons, err := p.getConsumer()
		if err != nil {
			return nil, err
		}

		p.consumer = cons
	}

	return p, nil
}

func (cl *Client) Close() error {
	if cl.nc != nil {
		return cl.nc.Drain()
	}
	return nil
}

func (cl *Client) getStream() (jetstream.Stream, error) {
	streamName := "nats-stream"
	cfg := jetstream.StreamConfig{
		Name:        streamName,
		Description: "Stream for microservice asynchronous messaging",
		Subjects:    []string{MinedTxsTopic, SubmitTxTopic, RequestTxTopic, RegisterTxTopic},
		Retention:   jetstream.WorkQueuePolicy,
		Discard:     jetstream.DiscardOld,
		MaxAge:      10 * time.Minute,
		Storage:     jetstream.MemoryStorage,
		NoAck:       false,
	}

	streamCtx, cancel := context.WithTimeout(cl.ctx, 60*time.Second)
	defer cancel()

	stream, err := cl.js.Stream(streamCtx, streamName)
	if errors.Is(err, jetstream.ErrStreamNotFound) {
		cl.logger.Error(fmt.Sprintf("stream %s not found, creating new", streamName))

		stream, err = cl.js.CreateStream(streamCtx, cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create stream: %v", err)
		}

		cl.logger.Info(fmt.Sprintf("stream %s created", streamName))
		return stream, nil
	} else if err != nil {
		return nil, err
	}

	cl.logger.Info(fmt.Sprintf("stream %s found", streamName))
	return stream, nil
}

func (cl *Client) getConsumer() (jetstream.Consumer, error) {
	consumerName := "nats-consumer"
	consCtx, cancel := context.WithTimeout(cl.ctx, 30*time.Second)
	defer cancel()

	cons, err := cl.stream.Consumer(consCtx, consumerName)
	if errors.Is(err, jetstream.ErrConsumerNotFound) {
		cl.logger.Error(fmt.Sprintf("consumer %s not found, creating new", consumerName))
		cons, err = cl.stream.CreateConsumer(consCtx, jetstream.ConsumerConfig{
			Durable:   consumerName,
			AckPolicy: jetstream.AckExplicitPolicy,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create consumer: %v", err)
		}

		cl.logger.Info(fmt.Sprintf("consumer %s created", consumerName))
		return cons, nil

	} else if err != nil {
		return nil, err
	}

	cl.logger.Info(fmt.Sprintf("consumer %s found", consumerName))
	return cons, err
}

func (cl *Client) Publish(topic string, hash []byte) error {
	_, err := cl.js.Publish(cl.ctx, topic, hash)
	if err != nil {
		return fmt.Errorf("failed to publish on %s topic: %w", topic, err)
	}

	return nil
}

func (cl *Client) PublishMarshal(topic string, m proto.Message) error {
	data, err := proto.Marshal(m)
	if err != nil {
		return err
	}

	err = cl.Publish(topic, data)
	if err != nil {
		return fmt.Errorf("failed to publish on %s topic: %w", topic, err)
	}

	return nil
}

func (cl *Client) Subscribe(topic string, msgFunc func([]byte) error) error {

	if !cl.hasConsumer || cl.consumer == nil {
		return errors.New("consumer not initialized")
	}

	_, err := cl.consumer.Consume(func(msg jetstream.Msg) {
		msgErr := msgFunc(msg.Data())
		if msgErr != nil {
			cl.logger.Error(fmt.Sprintf("failed to consume message on %s topic: %s", topic, string(msg.Data())))
			return
		}

		ackErr := msg.Ack()
		if ackErr != nil {
			cl.logger.Error(fmt.Sprintf("failed to acknowledge message on %s topic: %s", topic, string(msg.Data())))
		}
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to %s topic: %w", topic, err)
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
}
