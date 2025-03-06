package nats_jetstream

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"
)

type Client struct {
	js                             jetstream.JetStream
	nc                             *nats.Conn
	logger                         *slog.Logger
	consumers                      map[string]jetstream.Consumer
	storageType                    jetstream.StorageType
	ctx                            context.Context
	cancelAll                      context.CancelFunc
	removableStreamConsumerMapping map[string]string
	opts                           []Option
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

func WithStream(topic string, streamName string, retentionPolicy jetstream.RetentionPolicy, noAck bool) func(*Client) error {
	return func(cl *Client) error {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		// get or create stream for topic
		_, err := cl.js.Stream(ctx, streamName)
		if err != nil {
			if !errors.Is(err, jetstream.ErrStreamNotFound) {
				return errors.Join(ErrFailedToGetStream, err)
			}

			cl.logger.Info(fmt.Sprintf("stream %s not found, creating new", streamName))

			_, err = cl.js.CreateStream(ctx, jetstream.StreamConfig{
				Name:        streamName,
				Description: "Stream for topic " + topic,
				Subjects:    []string{topic},
				Retention:   retentionPolicy,
				Discard:     jetstream.DiscardOld,
				MaxAge:      10 * time.Minute,
				Storage:     cl.storageType,
				NoAck:       noAck,
			})
			if err != nil {
				return errors.Join(ErrFailedToCreateStream, err)
			}

			cl.logger.Info("stream created", slog.String("name", streamName))
		}

		return nil
	}
}

func WithConsumer(topic string, streamName string, consumerName string, durable bool, ackPolicy jetstream.AckPolicy) func(*Client) error {
	return func(cl *Client) error {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		// get or create consumer for topic
		cons, err := cl.js.Consumer(ctx, streamName, consumerName)
		if err != nil {
			if errors.Is(err, jetstream.ErrConsumerNotFound) {
				durableName := ""
				if durable {
					durableName = consumerName
				}

				_, err = cl.js.CreateConsumer(ctx, streamName, jetstream.ConsumerConfig{
					Name:          consumerName,
					Durable:       durableName,
					AckPolicy:     ackPolicy,
					MaxAckPending: 5000,
				})

				if err != nil {
					return errors.Join(ErrFailedToCreateConsumer, err)
				}

				cl.logger.Info(fmt.Sprintf("consumer %s created", consumerName))
			}

			return errors.Join(ErrFailedToGetConsumer, err)
		}
		cl.consumers[topic] = cons

		return nil
	}
}

func WithFileStorage() func(*Client) error {
	return func(c *Client) error {
		c.storageType = jetstream.FileStorage
		return nil
	}
}

type Option func(p *Client) error

func New(nc *nats.Conn, logger *slog.Logger, opts ...Option) (*Client, error) {
	ctx, cancel := context.WithCancel(context.Background())

	p := &Client{
		logger:                         logger.With("module", "nats-jetstream"),
		nc:                             nc,
		consumers:                      map[string]jetstream.Consumer{},
		storageType:                    jetstream.MemoryStorage,
		ctx:                            ctx,
		cancelAll:                      cancel,
		removableStreamConsumerMapping: map[string]string{},
		opts:                           opts,
	}

	js, err := jetstream.New(nc)
	if err != nil {
		return nil, err
	}

	p.js = js

	for _, opt := range opts {
		err = opt(p)
		if err != nil {
			return nil, errors.Join(ErrFailedToApplyOption, err)
		}
	}
	return p, nil
}

//
//func (cl *Client) createStream(topicName string, streamName string, retentionPolicy jetstream.RetentionPolicy, noAck bool) (jetstream.Stream, error) {
//	streamCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
//	defer cancel()
//
//	stream, err := cl.js.CreateStream(streamCtx, jetstream.StreamConfig{
//		Name:        streamName,
//		Description: "Stream for topic " + topicName,
//		Subjects:    []string{topicName},
//		Retention:   retentionPolicy,
//		Discard:     jetstream.DiscardOld,
//		MaxAge:      10 * time.Minute,
//		Storage:     cl.storageType,
//		NoAck:       noAck,
//	})
//	if err != nil {
//		return nil, errors.Join(ErrFailedToCreateStream, err)
//	}
//	cl.logger.Info(fmt.Sprintf("stream %s created", streamName))
//
//	return stream, nil
//}
//
//func (cl *Client) createConsumer(stream jetstream.Stream, ackPolicy jetstream.AckPolicy, consumerName string, durable bool) (jetstream.Consumer, error) {
//	consCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//
//	durableName := ""
//	if durable {
//		durableName = consumerName
//	}
//
//	cons, err := stream.CreateConsumer(consCtx, jetstream.ConsumerConfig{
//		Name:          consumerName,
//		Durable:       durableName,
//		AckPolicy:     ackPolicy,
//		MaxAckPending: 5000,
//	})
//	if err != nil {
//		return nil, errors.Join(ErrFailedToCreateConsumer, err)
//	}
//
//	cl.logger.Info(fmt.Sprintf("consumer %s created", consumerName))
//	return cons, nil
//}

func (cl *Client) Status() nats.Status {
	return cl.nc.Status()
}

func (cl *Client) getStream(topicName string, streamName string, retentionPolicy jetstream.RetentionPolicy) (jetstream.Stream, error) {
	streamCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	stream, err := cl.js.Stream(streamCtx, streamName)
	if errors.Is(err, jetstream.ErrStreamNotFound) {
		cl.logger.Info(fmt.Sprintf("stream %s not found, creating new", streamName))

		stream, err = cl.js.CreateStream(streamCtx, jetstream.StreamConfig{
			Name:        streamName,
			Description: "Stream for topic " + topicName,
			Subjects:    []string{topicName},
			Retention:   retentionPolicy,
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

	streamInfo, err := stream.Info(consCtx)
	if err != nil {
		return nil, fmt.Errorf("faied to get stream info: %w", err)
	}

	cons, err := stream.Consumer(consCtx, consumerName)
	if errors.Is(err, jetstream.ErrConsumerNotFound) {
		cl.logger.Warn("consumer not found, creating new", slog.String("consumer", consumerName), slog.String("stream", streamInfo.Config.Name))
		cons, err = stream.CreateConsumer(consCtx, jetstream.ConsumerConfig{
			Durable:       consumerName,
			AckPolicy:     jetstream.AckExplicitPolicy,
			MaxAckPending: 5000,
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
	_, err = cl.js.Publish(ctx, topic, hash)
	if err != nil {
		return errors.Join(ErrFailedToPublish, fmt.Errorf("topic: %s", topic), err)
	}

	return nil
}

func (cl *Client) PublishAsync(topic string, hash []byte) (err error) {
	_, err = cl.js.PublishAsync(topic, hash)
	if err != nil {
		return errors.Join(ErrFailedToPublish, fmt.Errorf("topic: %s", topic), err)
	}

	return nil
}

func (cl *Client) PublishMarshal(ctx context.Context, topic string, m proto.Message) (err error) {
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

func (cl *Client) PublishMarshalAsync(topic string, m proto.Message) (err error) {
	data, err := proto.Marshal(m)
	if err != nil {
		return err
	}

	err = cl.PublishAsync(topic, data)
	if err != nil {
		return errors.Join(ErrFailedToPublish, fmt.Errorf("topic: %s", topic), err)
	}

	return nil
}

func (cl *Client) SubscribeMsg(topic string, msgFunc func(msg jetstream.Msg) error) error {
	consumer, found := cl.consumers[topic]

	if !found {
		return ErrConsumerNotInitialized
	}

	_, err := consumer.Consume(func(msg jetstream.Msg) {
		msgErr := msgFunc(msg)
		if msgErr != nil {
			cl.logger.Error("failed to consume message", slog.String("topic", topic), slog.String("data", string(msg.Data())))
			return
		}
	})
	if err != nil {
		return errors.Join(ErrFailedToSubscribe, fmt.Errorf("topic: %s", topic), err)
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
	if cl == nil {
		return
	}

	if cl.removableStreamConsumerMapping != nil {
		for stream, consumer := range cl.removableStreamConsumerMapping {
			err := cl.js.DeleteConsumer(context.Background(), stream, consumer)
			if err != nil {
				cl.logger.Error("failed to delete consumer", slog.String("consumer", consumer), slog.String("stream", stream), slog.String("err", err.Error()))
			} else {
				cl.logger.Info("deleted consumer", slog.String("consumer", consumer), slog.String("stream", stream))
			}
		}
	}

	if cl.nc != nil {
		err := cl.nc.Drain()
		if err != nil {
			cl.logger.Error("failed to drain nats connection", slog.String("err", err.Error()))
		}
	}

	cl.cancelAll()
}
