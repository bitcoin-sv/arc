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
	return nil
}

func (m *MessageSubscribeAdapter) subscribeRegisterTx(registerTxChan chan []byte) error {
	err := m.mqClient.QueueSubscribe(mq.RegisterTxTopic, func(msg []byte) error {
		select {
		case registerTxChan <- msg:
		default:
			m.logger.Warn("Failed to send message on register tx channel")
		}

		return nil
	})
	if err != nil {
		return errors.Join(ErrFailedToSubscribeToTopic, fmt.Errorf(topic, mq.RegisterTxTopic), err)
	}

	return nil
}

func (m *MessageSubscribeAdapter) subscribeRegisterTxs(registerTxsChan chan []byte) error {
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
				m.logger.Warn("Failed to send message on register txs channel")
			}
		}

		return nil
	})
	if err != nil {
		return errors.Join(ErrFailedToSubscribeToTopic, fmt.Errorf(topic, mq.RegisterTxsTopic), err)
	}
	return nil
}

func (m *MessageSubscribeAdapter) Shutdown() {
	m.cancelAll()
	m.wg.Wait()
}
