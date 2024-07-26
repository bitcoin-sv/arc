package async

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

const (
	MinedTxsTopic = "mined-txs"
	minedTxsGroup = "mined-txs-group"
)

func (c MQClient) SubscribeMinedTxs() error {
	if c.minedTxsChan == nil {
		return errors.New("mined txs channel is nil")
	}

	subscription, err := c.nc.QueueSubscribe(MinedTxsTopic, minedTxsGroup, func(msg *nats.Msg) {
		serialized := &blocktx_api.TransactionBlock{}
		err := proto.Unmarshal(msg.Data, serialized)
		if err != nil {
			c.logger.Error("failed to unmarshal message", slog.String("err", err.Error()))
			return
		}

		c.minedTxsChan <- serialized
	})

	c.minedTxsSubscription = subscription

	if err != nil {
		return fmt.Errorf("failed to subscribe to %s topic: %w", MinedTxsTopic, err)
	}

	return nil
}
