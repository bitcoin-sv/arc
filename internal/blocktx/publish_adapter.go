package blocktx

import (
	"context"
	"log/slog"
	"sync"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
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

func (p *PublishAdapter) StartPublishMarshal(topic string, transactionBlocksChan chan *blocktx_api.TransactionBlocks) {
	p.wg.Go(func() {
		for {
			select {
			case <-p.ctx.Done():
				return
			case request := <-transactionBlocksChan:
				err := p.mqClient.PublishMarshalCore(topic, request)
				if err != nil {
					p.logger.Error("Failed to publish transaction blocks message", slog.Int("count", len(request.TransactionBlocks)), slog.String("err", err.Error()))
				}
			}
		}
	})
}

func (p *PublishAdapter) Shutdown() {
	p.cancelAll()
	p.wg.Wait()
}
