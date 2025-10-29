package metamorph

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/mq"
	"google.golang.org/protobuf/proto"
)

type MessageQueueAdapter struct {
	mqClient  mq.MessageQueueClient
	logger    *slog.Logger
	ctx       context.Context
	cancelAll context.CancelFunc
	wg        *sync.WaitGroup
}

func NewMessageQueueAdapter(mqClient mq.MessageQueueClient, logger *slog.Logger) *MessageQueueAdapter {
	m := &MessageQueueAdapter{
		mqClient: mqClient,
		logger:   logger,
		wg:       &sync.WaitGroup{},
	}

	m.ctx, m.cancelAll = context.WithCancel(context.Background())

	return m
}

func (m *MessageQueueAdapter) Start(
	minedTxsChan chan *blocktx_api.TransactionBlocks,
	submittedTxsChan chan *metamorph_api.PostTransactionRequest,
	callbackChan chan *callbacker_api.SendRequest,
	registerTxChan chan []byte,
	registerTxsChan chan *blocktx_api.Transactions,
) error {
	err := m.subscribeMinedTxs(minedTxsChan)
	if err != nil {
		return fmt.Errorf("failed to start mined txs: %w", err)
	}
	err = m.subscribeSubmitTxs(submittedTxsChan)
	if err != nil {
		return fmt.Errorf("failed to start submit txs: %w", err)
	}
	m.startPublishCallbacks(callbackChan)
	m.startPublishRegisterTx(registerTxChan)
	m.startPublishRegisterTxs(registerTxsChan)
	return nil
}

func (m *MessageQueueAdapter) subscribeMinedTxs(minedTxsChan chan *blocktx_api.TransactionBlocks) error {
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

func (m *MessageQueueAdapter) subscribeSubmitTxs(submittedTxsChan chan *metamorph_api.PostTransactionRequest) error {
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

func (m *MessageQueueAdapter) startPublishCallbacks(callbackChan chan *callbacker_api.SendRequest) {
	m.wg.Go(func() {
		for {
			select {
			case <-m.ctx.Done():
				return
			case request := <-callbackChan:
				err := m.mqClient.PublishMarshal(m.ctx, mq.CallbackTopic, request)
				if err != nil {
					m.logger.Error("Failed to publish callback", slog.String("err", err.Error()))
				}
			}
		}
	})
}

func (m *MessageQueueAdapter) startPublishRegisterTx(registerTxChan chan []byte) {
	m.wg.Go(func() {
		for {
			select {
			case <-m.ctx.Done():
				return
			case request := <-registerTxChan:
				err := m.mqClient.PublishCore(mq.RegisterTxTopic, request)
				if err != nil {
					m.logger.Error("Failed to publish callback", slog.String("err", err.Error()))
				}
			}
		}
	})
}

func (m *MessageQueueAdapter) startPublishRegisterTxs(registerTxsChan chan *blocktx_api.Transactions) {
	m.wg.Go(func() {
		for {
			select {
			case <-m.ctx.Done():
				return
			case request := <-registerTxsChan:

				err := m.mqClient.PublishMarshal(m.ctx, mq.RegisterTxsTopic, request)
				if err != nil {
					m.logger.Error("Failed to publish callback", slog.String("err", err.Error()))
				}
			}
		}
	})
}

func (m *MessageQueueAdapter) Shutdown() {
	m.cancelAll()
	m.wg.Wait()
}
