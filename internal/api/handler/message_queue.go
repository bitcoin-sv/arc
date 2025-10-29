package handler

import (
	"context"
	"log/slog"
	"sync"

	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/mq"
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

func (m *MessageQueueProvider) Start(submitTxsCh chan *metamorph_api.PostTransactionRequest) {
	m.wg.Go(func() {
		for {
			select {
			case <-m.ctx.Done():
				return
			case request := <-submitTxsCh:
				err := m.mqClient.PublishMarshal(m.ctx, mq.SubmitTxTopic, request)
				if err != nil {
					m.logger.Error("Failed to publish callback", slog.String("err", err.Error()))
				}
			}
		}
	})
}
