package blocktx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/mq"
	"google.golang.org/protobuf/proto"
)

type MessageQueueProvider struct {
	mqClient  mq.MessageQueueClient
	logger    *slog.Logger
	ctx       context.Context
	cancelAll context.CancelFunc
	wg        *sync.WaitGroup
}

func NewMessageQueueProvider(mqClient mq.MessageQueueClient, logger *slog.Logger) *MessageQueueProvider {
	m := &MessageQueueProvider{
		mqClient: mqClient,
		logger:   logger,
		wg:       &sync.WaitGroup{},
	}

	m.ctx, m.cancelAll = context.WithCancel(context.Background())

	return m
}

func (m *MessageQueueProvider) Start(
	minedTxsChan chan *blocktx_api.TransactionBlocks,
	registerTxChan chan []byte,
) error {
	err := m.subscribeRegisterTx(registerTxChan)
	if err != nil {
		return fmt.Errorf("failed to start mined txs: %w", err)
	}
	err = m.subscribeRegisterTxs(registerTxChan)
	if err != nil {
		return fmt.Errorf("failed to start submit txs: %w", err)
	}
	m.startPublishMinedTxs(minedTxsChan)
	return nil
}

func (m *MessageQueueProvider) subscribeRegisterTx(registerTxChan chan []byte) error {
	err := m.mqClient.QueueSubscribe(mq.RegisterTxTopic, func(msg []byte) error {
		select {
		case registerTxChan <- msg:
		default:
		}

		return nil
	})
	if err != nil {
		return errors.Join(ErrFailedToSubscribeToTopic, fmt.Errorf(topic, mq.RegisterTxTopic), err)
	}

	return nil
}

func (m *MessageQueueProvider) subscribeRegisterTxs(registerTxsChan chan []byte) error {
	err := m.mqClient.QueueSubscribe(mq.RegisterTxsTopic, func(msg []byte) error {
		serialized := &blocktx_api.Transactions{}
		err := proto.Unmarshal(msg, serialized)
		if err != nil {
			return errors.Join(ErrFailedToUnmarshalMessage, fmt.Errorf(topic, mq.RegisterTxsTopic), err)
		}

		for _, tx := range serialized.Transactions {
			select {
			case registerTxsChan <- tx.Hash:
			default:
			}
		}

		return nil
	})
	if err != nil {
		return errors.Join(ErrFailedToSubscribeToTopic, fmt.Errorf(topic, mq.RegisterTxsTopic), err)
	}
	return nil
}

func (m *MessageQueueProvider) startPublishMinedTxs(minedTxsChan chan *blocktx_api.TransactionBlocks) {
	m.wg.Go(func() {
		for {
			select {
			case <-m.ctx.Done():
				return
			case request := <-minedTxsChan:
				err := m.mqClient.PublishMarshal(m.ctx, mq.MinedTxsTopic, request)
				if err != nil {
					m.logger.Error("Failed to publish callback", slog.String("err", err.Error()))
				}
			}
		}
	})
}
