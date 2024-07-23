package async

import (
	"log/slog"

	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

const (
	registerTxTopic = "register-tx"
	requestTxTopic  = "request-tx"

	consumerQueue = "mined-txs-group"
	minedTxsTopic = "mined-txs"

	submitTxTopic = "submit-tx"
)

type NatsClient interface {
	QueueSubscribe(subj, queue string, cb nats.MsgHandler) (*nats.Subscription, error)
	Close()
	Publish(subj string, data []byte) error
	Drain() error
}

type MQClient struct {
	nc               NatsClient
	logger           *slog.Logger
	minedTxsChan     chan *blocktx_api.TransactionBlocks
	submittedTxsChan chan *metamorph_api.TransactionRequest
	subscription     *nats.Subscription
}

func NewNatsMQClient(nc NatsClient, minedTxsChan chan *blocktx_api.TransactionBlocks, submittedTxsChan chan *metamorph_api.TransactionRequest, logger *slog.Logger) metamorph.MessageQueueClient {
	return &MQClient{nc: nc, logger: logger, minedTxsChan: minedTxsChan, submittedTxsChan: submittedTxsChan}
}

func (c MQClient) PublishRegisterTxs(hash []byte) error {
	err := c.nc.Publish(registerTxTopic, hash)
	if err != nil {
		return err
	}

	return nil
}

func (c MQClient) PublishRequestTx(hash []byte) error {
	err := c.nc.Publish(requestTxTopic, hash)
	if err != nil {
		return err
	}

	return nil
}

func (c MQClient) SubscribeSubmittedTx() error {
	subscription, err := c.nc.QueueSubscribe(submitTxTopic, consumerQueue, func(msg *nats.Msg) {
		serialized := &metamorph_api.TransactionRequest{}
		err := proto.Unmarshal(msg.Data, serialized)
		if err != nil {
			c.logger.Error("failed to unmarshal message", slog.String("err", err.Error()))
			return
		}

		c.submittedTxsChan <- serialized
	})

	c.subscription = subscription

	if err != nil {
		return err
	}

	return nil
}

func (c MQClient) SubscribeMinedTxs() error {
	subscription, err := c.nc.QueueSubscribe(minedTxsTopic, consumerQueue, func(msg *nats.Msg) {
		serialized := &blocktx_api.TransactionBlocks{}
		err := proto.Unmarshal(msg.Data, serialized)
		if err != nil {
			c.logger.Error("failed to unmarshal message", slog.String("err", err.Error()))
			return
		}

		c.minedTxsChan <- serialized
	})

	c.subscription = subscription

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

	if c.minedTxsChan != nil {
		close(c.minedTxsChan)
	}

	c.nc.Close()

	return nil
}
