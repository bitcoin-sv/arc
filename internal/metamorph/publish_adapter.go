package metamorph

import (
	"context"
	"log/slog"
	"sync"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
	"github.com/bitcoin-sv/arc/internal/mq"
)

type PublishAdapter struct {
	mqClient  mq.MessageQueueClient
	logger    *slog.Logger
	ctx       context.Context
	cancelAll context.CancelFunc
	wg        *sync.WaitGroup
}

func NewPublishAdapter(mqClient mq.MessageQueueClient, logger *slog.Logger) *PublishAdapter {
	m := &PublishAdapter{
		mqClient: mqClient,
		logger:   logger,
		wg:       &sync.WaitGroup{},
	}

	m.ctx, m.cancelAll = context.WithCancel(context.Background())

	return m
}

func (p *PublishAdapter) StartPublishBlockTransactions(topic string, messageChan chan *blocktx_api.Transactions) {
	p.wg.Go(func() {
		for {
			select {
			case <-p.ctx.Done():
				return
			case request := <-messageChan:
				err := p.mqClient.PublishMarshalCore(topic, request)
				if err != nil {
					p.logger.Error("Failed to publish transactions message", slog.String("err", err.Error()))
				}
			}
		}
	})
}

func (p *PublishAdapter) StartPublishSendRequests(topic string, messageChan chan *callbacker_api.SendRequest) {
	p.wg.Go(func() {
		for {
			select {
			case <-p.ctx.Done():
				return
			case request := <-messageChan:
				err := p.mqClient.PublishMarshal(p.ctx, topic, request)
				if err != nil {
					p.logger.Error("Failed to publish send request message", slog.String("err", err.Error()))
				}
			}
		}
	})
}

func (p *PublishAdapter) StartPublishCore(topic string, msgChan chan []byte) {
	p.wg.Go(func() {
		for {
			select {
			case <-p.ctx.Done():
				return
			case request := <-msgChan:
				err := p.mqClient.PublishCore(topic, request)
				if err != nil {
					p.logger.Error("Failed to publish message", slog.String("err", err.Error()))
				}
			}
		}
	})
}

func (p *PublishAdapter) Shutdown() {
	p.cancelAll()
	p.wg.Wait()
}
