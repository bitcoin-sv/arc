package async

import (
	"github.com/bitcoin-sv/arc/pkg/metamorph"
	"github.com/bitcoin-sv/arc/pkg/metamorph/metamorph_api"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

const (
	submitTxTopic = "submit-tx"
)

type NatsClient interface {
	QueueSubscribe(subj, queue string, cb nats.MsgHandler) (*nats.Subscription, error)
	Close()
	Publish(subj string, data []byte) error
	Drain() error
}

type MQClient struct {
	nc NatsClient
}

func NewNatsMQClient(nc NatsClient) metamorph.MessageQueueClient {
	return &MQClient{nc: nc}
}

func (c MQClient) PublishSubmitTx(tx *metamorph_api.TransactionRequest) error {

	data, err := proto.Marshal(tx)
	if err != nil {
		return err
	}

	err = c.nc.Publish(submitTxTopic, data)
	if err != nil {
		return err
	}

	return nil
}

func (c MQClient) PublishSubmitTxs(txs *metamorph_api.TransactionRequests) error {
	for _, tx := range txs.Transactions {
		err := c.PublishSubmitTx(tx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c MQClient) Shutdown() error {

	err := c.nc.Drain()
	if err != nil {
		return err
	}

	c.nc.Close()

	return nil
}
