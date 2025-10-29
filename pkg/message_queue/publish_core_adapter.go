package message_queue

import (
	"context"
	"log/slog"
	"sync"

	"github.com/bitcoin-sv/arc/internal/mq"
)

type PublishCoreAdapter struct {
	mqClient         mq.MessageQueueClient
	logger           *slog.Logger
	ctx              context.Context
	cancelAll        context.CancelFunc
	wg               *sync.WaitGroup
	messageBytesChan chan []byte
}

func NewPublishCoreAdapter(mqClient mq.MessageQueueClient, logger *slog.Logger) *PublishCoreAdapter {
	m := &PublishCoreAdapter{
		mqClient:         mqClient,
		logger:           logger,
		wg:               &sync.WaitGroup{},
		messageBytesChan: make(chan []byte, 500),
	}

	m.ctx, m.cancelAll = context.WithCancel(context.Background())

	return m
}

func (p *PublishCoreAdapter) StartPublish(topic string) {
	p.wg.Go(func() {
		for {
			select {
			case <-p.ctx.Done():
				return
			case request := <-p.messageBytesChan:
				err := p.mqClient.PublishCore(topic, request)
				if err != nil {
					p.logger.Error("Failed to publish callback", slog.String("err", err.Error()))
				}
			}
		}
	})
}

func (p *PublishCoreAdapter) PublishCore(msg []byte) {
	p.messageBytesChan <- msg
}

func (p *PublishCoreAdapter) Shutdown() {
	p.cancelAll()
	p.wg.Wait()
}
