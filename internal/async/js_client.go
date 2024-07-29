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
	url       string
	js        jetstream.JetStream
	nc        NatsClient
	logger    *slog.Logger
	consumers map[string]jetstream.Consumer
	ctx       context.Context
}

func WithSubscribedTopics(topics ...string) func(handler *Client) error {
	return func(c *Client) error {
		for _, topic := range topics {
			streamMinedTxs, err := c.getStream(topic, fmt.Sprintf("%s-stream", topic))
			if err != nil {
				return fmt.Errorf("failed to get stream: %v", err)
			}

			cons, err := c.getConsumer(streamMinedTxs, fmt.Sprintf("%s-cons", topic))
			if err != nil {
				return fmt.Errorf("failed to get consumer: %v", err)
			}

			c.consumers[topic] = cons
		}

		return nil
	}
}

func NewJetStreamClient(ctx context.Context, nc *nats.Conn, logger *slog.Logger, url string, topics []string, opts ...func(client *Client) error) (*Client, error) {

	p := &Client{
		logger:    logger,
		url:       url,
		ctx:       ctx,
		nc:        nc,
		consumers: map[string]jetstream.Consumer{},
	}

	js, err := jetstream.New(nc)
	if err != nil {
		return nil, err
	}

	p.js = js

	for _, topic := range topics {
		_, err = p.getStream(topic, fmt.Sprintf("%s-stream", topic))
		if err != nil {
			return nil, fmt.Errorf("failed to get stream: %v", err)
		}
	}

	for _, opt := range opts {
		err = opt(p)
		if err != nil {
			return nil, fmt.Errorf("failed to apply option: %v", err)
		}
	}
	return p, nil
}

func (cl *Client) Close() error {
	if cl.nc != nil {
		return cl.nc.Drain()
	}
	return nil
}

func (cl *Client) getStream(topicName string, streamName string) (jetstream.Stream, error) {

	streamCtx, cancel := context.WithTimeout(cl.ctx, 60*time.Second)
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
			Storage:     jetstream.MemoryStorage,
			NoAck:       false,
		})
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

func (cl *Client) getConsumer(stream jetstream.Stream, consumerName string) (jetstream.Consumer, error) {
	consCtx, cancel := context.WithTimeout(cl.ctx, 30*time.Second)
	defer cancel()

	cons, err := stream.Consumer(consCtx, consumerName)
	if errors.Is(err, jetstream.ErrConsumerNotFound) {
		cl.logger.Error(fmt.Sprintf("consumer %s not found, creating new", consumerName))
		cons, err = stream.CreateConsumer(consCtx, jetstream.ConsumerConfig{
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
	consumer, found := cl.consumers[topic]

	if !found {
		return errors.New("consumer not initialized")
	}

	_, err := consumer.Consume(func(msg jetstream.Msg) {
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
