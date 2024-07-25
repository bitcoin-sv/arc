package async

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

const (
	SubmitTxTopic = "submit-tx"
	SubmitTxGroup = "submit-tx-group"
)

func (c MQClient) PublishSubmitTx(tx *metamorph_api.TransactionRequest) error {
	data, err := proto.Marshal(tx)
	if err != nil {
		return err
	}

	err = c.nc.Publish(SubmitTxTopic, data)
	if err != nil {
		return fmt.Errorf("failed to publish on %s topic %w", SubmitTxTopic, err)
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

func (c MQClient) SubscribeSubmittedTx() error {
	if c.submittedTxsChan == nil {
		return errors.New("submitted txs channel is nil")
	}

	subscription, err := c.nc.QueueSubscribe(SubmitTxTopic, SubmitTxGroup, func(msg *nats.Msg) {
		serialized := &metamorph_api.TransactionRequest{}
		err := proto.Unmarshal(msg.Data, serialized)
		if err != nil {
			c.logger.Error("failed to unmarshal message", slog.String("err", err.Error()))
			return
		}

		c.submittedTxsChan <- serialized
	})

	c.minedTxsSubscription = subscription

	if err != nil {
		return fmt.Errorf("failed to subscribe to %s topic: %w", SubmitTxTopic, err)
	}

	return nil
}
