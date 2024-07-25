package async

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"
)

const (
	MinedTxsTopic = "mined-txs"
	minedTxsGroup = "mined-txs-group"
)

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

	err = c.nc.Publish(MinedTxsTopic, data)
	if err != nil {
		return fmt.Errorf("failed to publish on %s topic: %w", MinedTxsTopic, err)
	}

	return nil
}

func (c MQClient) SubscribeMinedTxs() error {
	if c.minedTxsChan == nil {
		return errors.New("mined txs channel is nil")
	}

	subscription, err := c.nc.QueueSubscribe(MinedTxsTopic, minedTxsGroup, func(msg *nats.Msg) {
		serialized := &blocktx_api.TransactionBlocks{}
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
