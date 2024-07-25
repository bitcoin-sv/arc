package async

import (
	"errors"
	"fmt"

	"github.com/nats-io/nats.go"
)

const (
	RequestTxTopic = "request-tx"
	requestTxGroup = "request-tx-group"
)

func (c MQClient) SubscribeRequestTxs() error {
	if c.requestTxChannel == nil {
		return errors.New("request txs channel is nil")
	}

	subscription, err := c.nc.QueueSubscribe(RequestTxTopic, requestTxGroup, func(msg *nats.Msg) {
		c.requestTxChannel <- msg.Data
	})

	c.requestSubscription = subscription

	if err != nil {
		return fmt.Errorf("failed to subscribe to %s topic: %w", RequestTxTopic, err)
	}

	return nil
}
