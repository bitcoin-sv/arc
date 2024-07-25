package async

import (
	"errors"
	"fmt"

	"github.com/nats-io/nats.go"
)

const (
	RegisterTxTopic = "register-tx"
	registerTxGroup = "register-tx-group"
)

func (c MQClient) SubscribeRegisterTxs() error {
	if c.registerTxsChannel == nil {
		return errors.New("register txs channel is nil")
	}

	subscription, err := c.nc.QueueSubscribe(RegisterTxTopic, registerTxGroup, func(msg *nats.Msg) {
		c.registerTxsChannel <- msg.Data
	})

	c.registerTxsSubscription = subscription

	if err != nil {
		return fmt.Errorf("failed to subscribe to %s topic: %w", RegisterTxTopic, err)
	}

	return nil
}
