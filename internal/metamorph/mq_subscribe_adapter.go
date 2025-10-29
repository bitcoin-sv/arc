package metamorph

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/mq"
	"google.golang.org/protobuf/proto"
)

type MessageSubscribeAdapter struct {
	mqClient  mq.MessageQueueClient
	logger    *slog.Logger
	ctx       context.Context
	cancelAll context.CancelFunc
	wg        *sync.WaitGroup
}

func NewMessageSubscribeAdapter(mqClient mq.MessageQueueClient, logger *slog.Logger) *MessageSubscribeAdapter {
	m := &MessageSubscribeAdapter{
		mqClient: mqClient,
		logger:   logger,
		wg:       &sync.WaitGroup{},
	}

	m.ctx, m.cancelAll = context.WithCancel(context.Background())

	return m
}

func (m *MessageSubscribeAdapter) Start(
	minedTxsChan chan *blocktx_api.TransactionBlocks,
	submittedTxsChan chan *metamorph_api.PostTransactionRequest,

) error {
	err := m.subscribeMinedTxs(minedTxsChan)
	if err != nil {
		return fmt.Errorf("failed to start mined txs: %w", err)
	}
	err = m.subscribeSubmitTxs(submittedTxsChan)
	if err != nil {
		return fmt.Errorf("failed to start submit txs: %w", err)
	}

	return nil
}

func (m *MessageSubscribeAdapter) subscribeMinedTxs(minedTxsChan chan *blocktx_api.TransactionBlocks) error {
	err := m.mqClient.QueueSubscribe(mq.MinedTxsTopic, func(msg []byte) error {
		serialized := &blocktx_api.TransactionBlocks{}
		err := proto.Unmarshal(msg, serialized)
		if err != nil {
			return errors.Join(ErrFailedToUnmarshalMessage, fmt.Errorf("subscribed on %s topic", mq.MinedTxsTopic), err)
		}

		minedTxsChan <- serialized
		return nil
	})

	if err != nil {
		return errors.Join(ErrFailedToSubscribe, fmt.Errorf("to %s topic", mq.MinedTxsTopic), err)
	}
	return nil
}

func (m *MessageSubscribeAdapter) subscribeSubmitTxs(submittedTxsChan chan *metamorph_api.PostTransactionRequest) error {
	err := m.mqClient.Consume(mq.SubmitTxTopic, func(msg []byte) error {
		serialized := &metamorph_api.PostTransactionRequest{}
		marshalErr := proto.Unmarshal(msg, serialized)
		if marshalErr != nil {
			return errors.Join(ErrFailedToUnmarshalMessage, fmt.Errorf("subscribed on %s topic", mq.SubmitTxTopic), marshalErr)
		}
		submittedTxsChan <- serialized
		return nil
	})
	if err != nil {
		m.logger.Warn("Failed to start consuming from topic", slog.String("topic", mq.SubmitTxTopic), slog.String("err", err.Error()))
		errSubscribe := m.mqClient.QueueSubscribe(mq.SubmitTxTopic, func(msg []byte) error {
			serialized := &metamorph_api.PostTransactionRequest{}
			marshalErr := proto.Unmarshal(msg, serialized)
			if marshalErr != nil {
				return errors.Join(ErrFailedToUnmarshalMessage, fmt.Errorf("subscribed on %s topic", mq.SubmitTxTopic), marshalErr)
			}
			submittedTxsChan <- serialized
			return nil
		})
		if errSubscribe != nil {
			return errors.Join(ErrFailedToSubscribe, fmt.Errorf("to %s topic", mq.SubmitTxTopic), err)
		}
	}
	return nil
}

func (m *MessageSubscribeAdapter) Shutdown() {
	m.cancelAll()
	m.wg.Wait()
}
