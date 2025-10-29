package callbacker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
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
	sendRequestCh chan *callbacker_api.SendRequest,
) error {
	err := m.subscribeCallback(sendRequestCh)
	if err != nil {
		return fmt.Errorf("failed to start submit txs: %w", err)
	}
	return nil
}

func (m *MessageSubscribeAdapter) subscribeCallback(sendRequestCh chan *callbacker_api.SendRequest) error {
	err := m.mqClient.Consume(mq.CallbackTopic, func(msg []byte) error {
		serialized := &callbacker_api.SendRequest{}
		err := proto.Unmarshal(msg, serialized)
		if err != nil {
			return fmt.Errorf("failed to unmarshal send request on %s topic", mq.CallbackTopic)
		}

		m.logger.Debug("Enqueued callback request",
			slog.String("url", serialized.CallbackRouting.Url),
			slog.String("token", serialized.CallbackRouting.Token),
			slog.String("hash", serialized.Txid),
			slog.String("status", serialized.Status.String()),
		)

		sendRequestCh <- serialized

		return nil
	})
	if err != nil {
		return errors.Join(ErrConsume, fmt.Errorf("failed to consume topic %s: %v", mq.CallbackTopic, err))
	}
	return nil
}

func (m *MessageSubscribeAdapter) Shutdown() {
	m.cancelAll()
	m.wg.Wait()
}
