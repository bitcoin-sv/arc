package message_queue

import (
	"context"
	"log/slog"
	"sync"

	"github.com/bitcoin-sv/arc/internal/mq"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type PublishAdapter struct {
	mqClient    mq.MessageQueueClient
	logger      *slog.Logger
	ctx         context.Context
	cancelAll   context.CancelFunc
	wg          *sync.WaitGroup
	messageChan chan protoreflect.ProtoMessage
}

func NewPublishAdapter(mqClient mq.MessageQueueClient, logger *slog.Logger) *PublishAdapter {
	m := &PublishAdapter{
		mqClient:    mqClient,
		logger:      logger,
		wg:          &sync.WaitGroup{},
		messageChan: make(chan protoreflect.ProtoMessage, 500),
	}

	m.ctx, m.cancelAll = context.WithCancel(context.Background())

	return m
}

func (p *PublishAdapter) StartPublishMarshal(topic string) {
	p.wg.Go(func() {
		for {
			select {
			case <-p.ctx.Done():
				return
			case request := <-p.messageChan:
				err := p.mqClient.PublishMarshal(p.ctx, topic, request)
				if err != nil {
					p.logger.Error("Failed to publish marshal message", slog.String("err", err.Error()))
				}
			}
		}
	})
}

func (p *PublishAdapter) Publish(msg protoreflect.ProtoMessage) {
	p.messageChan <- msg
}

func (p *PublishAdapter) Shutdown() {
	p.cancelAll()
	p.wg.Wait()
}
