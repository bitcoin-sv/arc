package nats_mq

import (
	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/pkg/blocktx/blocktx_api"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

const (
	consumerQueue       = "register-tx-group"
	registerTxTopic     = "register-tx"
	requestTxTopic      = "request-tx"
	minedTxsTopic       = "mined-txs"
	connectionTries     = 5
	maxBatchSizeDefault = 20
)

func WithMaxBatchSize(size int) func(*MQClient) {
	return func(m *MQClient) {
		m.maxBatchSize = size
	}
}

type MQClient struct {
	nc                  NatsClient
	txChannel           chan []byte
	requestTxChannel    chan []byte
	subscription        *nats.Subscription
	requestSubscription *nats.Subscription
	maxBatchSize        int
}

type NatsClient interface {
	QueueSubscribe(subj, queue string, cb nats.MsgHandler) (*nats.Subscription, error)
	Close()
	Publish(subj string, data []byte) error
	Drain() error
}

func NewNatsMQClient(nc NatsClient, txChannel chan []byte, requestTxChannel chan []byte, opts ...func(client *MQClient)) blocktx.MessageQueueClient {
	m := &MQClient{nc: nc, txChannel: txChannel, requestTxChannel: requestTxChannel, maxBatchSize: maxBatchSizeDefault}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

func (c MQClient) SubscribeRegisterTxs() error {

	subscription, err := c.nc.QueueSubscribe(registerTxTopic, consumerQueue, func(msg *nats.Msg) {
		c.txChannel <- msg.Data
	})

	c.subscription = subscription

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

func (c MQClient) PublishMinedTxs(txsBlocks []*blocktx_api.TransactionBlock) error {
	txBlockBatch := make([]*blocktx_api.TransactionBlock, 0, c.maxBatchSize)
	for i, txBlock := range txsBlocks {
		txBlockBatch = append(txBlockBatch, txBlock)
		if (i+1)%c.maxBatchSize == 0 {
			err := c.publish(txBlockBatch)
			if err != nil {
				return err
			}
			txBlockBatch = make([]*blocktx_api.TransactionBlock, 0, c.maxBatchSize)
		}
	}

	if len(txBlockBatch) == 0 {
		return nil
	}

	err := c.publish(txBlockBatch)
	if err != nil {
		return err
	}

	return nil
}

func (c MQClient) publish(txBlockBatch []*blocktx_api.TransactionBlock) error {
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
	err := c.subscription.Unsubscribe()
	if err != nil {
		return err
	}

	err = c.requestSubscription.Unsubscribe()
	if err != nil {
		return err
	}

	c.nc.Close()

	err = c.nc.Drain()
	if err != nil {
		return err
	}

	return nil
}
