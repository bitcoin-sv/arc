package async

import (
	"context"
	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/pkg/blocktx/blocktx_api"
	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"
)

const (
	consumerQueue       = "register-tx-group"
	registerTxTopic     = "register-tx"
	requestTxTopic      = "request-tx"
	minedTxsTopic       = "mined-txs"
	maxBatchSizeDefault = 20
)

var tracer trace.Tracer

func WithMaxBatchSize(size int) func(*MQClient) {
	return func(m *MQClient) {
		m.maxBatchSize = size
	}
}

func WithTracer() func(handler *MQClient) {
	return func(_ *MQClient) {
		tracer = otel.GetTracerProvider().Tracer("")
	}
}

type MQClient struct {
	nc                      NatsClient
	registerTxsChannel      chan []byte
	requestTxChannel        chan []byte
	registerTxsSubscription *nats.Subscription
	requestSubscription     *nats.Subscription
	maxBatchSize            int
}

type NatsClient interface {
	QueueSubscribe(subj, queue string, cb nats.MsgHandler) (*nats.Subscription, error)
	Close()
	Publish(subj string, data []byte) error
	Drain() error
}

func NewNatsMQClient(nc NatsClient, registerTxsChannel chan []byte, requestTxChannel chan []byte, opts ...func(client *MQClient)) blocktx.MessageQueueClient {
	m := &MQClient{nc: nc, registerTxsChannel: registerTxsChannel, requestTxChannel: requestTxChannel, maxBatchSize: maxBatchSizeDefault}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

func (c MQClient) SubscribeRegisterTxs() error {

	subscription, err := c.nc.QueueSubscribe(registerTxTopic, consumerQueue, func(msg *nats.Msg) {
		c.registerTxsChannel <- msg.Data
	})

	c.registerTxsSubscription = subscription

	if err != nil {
		return err
	}

	return nil
}

func (c MQClient) SubscribeRequestTxs() error {
	subscription, err := c.nc.QueueSubscribe(requestTxTopic, consumerQueue, func(msg *nats.Msg) {
		c.requestTxChannel <- msg.Data
	})

	c.requestSubscription = subscription

	if err != nil {
		return err
	}

	return nil
}

func (c MQClient) PublishMinedTxs(ctx context.Context, txsBlocks []*blocktx_api.TransactionBlock) error {
	if tracer != nil {
		var span trace.Span
		_, span = tracer.Start(ctx, "PublishMinedTxs")
		defer span.End()
	}

	txBlockBatch := make([]*blocktx_api.TransactionBlock, 0, c.maxBatchSize)
	for i, txBlock := range txsBlocks {
		txBlockBatch = append(txBlockBatch, txBlock)
		if (i+1)%c.maxBatchSize == 0 {
			err := c.publishMinedTxs(txBlockBatch)
			if err != nil {
				return err
			}
			txBlockBatch = make([]*blocktx_api.TransactionBlock, 0, c.maxBatchSize)
		}
	}

	if len(txBlockBatch) == 0 {
		return nil
	}

	err := c.publishMinedTxs(txBlockBatch)
	if err != nil {
		return err
	}

	return nil
}

func (c MQClient) publishMinedTxs(txBlockBatch []*blocktx_api.TransactionBlock) error {
	data, err := proto.Marshal(&blocktx_api.TransactionBlocks{TransactionBlocks: txBlockBatch})
	if err != nil {
		return err
	}

	err = c.nc.Publish(minedTxsTopic, data)
	if err != nil {
		return err
	}

	return nil
}

func (c MQClient) Shutdown() error {

	err := c.nc.Drain()
	if err != nil {
		return err
	}

	if c.registerTxsChannel != nil {
		close(c.registerTxsChannel)
	}

	if c.requestTxChannel != nil {
		close(c.requestTxChannel)
	}

	return nil
}
