package callbacker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"

	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
	"github.com/bitcoin-sv/arc/internal/callbacker/store"
)

type Dispatcher interface {
	Dispatch(url string, dto *CallbackEntry, allowBatch bool)
}

const (
	syncInterval = 5 * time.Second
)

var (
	ErrUnmarshal  = errors.New("failed to unmarshal message")
	ErrNakMessage = errors.New("failed to nak message")
	ErrAckMessage = errors.New("failed to ack message")
	ErrSetMapping = errors.New("failed to set mapping")
)

type Processor struct {
	mqClient   MessageQueueClient
	dispatcher Dispatcher
	store      store.ProcessorStore
	logger     *slog.Logger

	hostName  string
	waitGroup *sync.WaitGroup
	cancelAll context.CancelFunc
	ctx       context.Context

	mu         sync.RWMutex
	urlMapping map[string]string
}

func NewProcessor(dispatcher Dispatcher, processorStore store.ProcessorStore, mqClient MessageQueueClient, hostName string, logger *slog.Logger, opts ...func(*Processor)) (*Processor, error) {
	p := &Processor{
		hostName:   hostName,
		urlMapping: make(map[string]string),
		dispatcher: dispatcher,
		waitGroup:  &sync.WaitGroup{},
		store:      processorStore,
		logger:     logger,
		mqClient:   mqClient,
	}
	for _, opt := range opts {
		opt(p)
	}

	ctx, cancelAll := context.WithCancel(context.Background())
	p.cancelAll = cancelAll
	p.ctx = ctx

	return p, nil
}

func (p *Processor) handleCallbackMessage(msg jetstream.Msg) error {
	p.logger.Debug("message received", "instance", p.hostName)

	request := &callbacker_api.SendRequest{}
	err := proto.Unmarshal(msg.Data(), request)
	if err != nil {
		nakErr := msg.Nak()
		if nakErr != nil {
			return errors.Join(errors.Join(ErrUnmarshal, err), errors.Join(ErrNakMessage, nakErr))
		}
		return errors.Join(ErrUnmarshal, err)
	}

	// check if this processor the first URL of this request is mapped to this instance
	p.mu.Lock()
	instance, found := p.urlMapping[(request.CallbackRouting.Url)]
	p.mu.Unlock()
	if !found {
		p.logger.Debug("setting URL mapping", "instance", p.hostName, "url", request.CallbackRouting.Url)
		err = p.store.SetURLMapping(context.Background(), store.URLMapping{
			URL:      request.CallbackRouting.Url,
			Instance: p.hostName,
		})
		if err != nil {
			if errors.Is(err, store.ErrURLMappingDuplicateKey) {
				p.logger.Debug("URL already mapped", slog.String("err", err.Error()))

				errAck := msg.Ack()
				if errAck != nil {
					return errors.Join(ErrAckMessage, errAck)
				}

				return nil
			}

			p.logger.Error("failed to set URL mapping", slog.String("err", err.Error()))

			nakErr := msg.Nak()
			if nakErr != nil {
				return errors.Join(errors.Join(ErrSetMapping, err), errors.Join(ErrNakMessage, nakErr))
			}
			return errors.Join(ErrSetMapping, err)
		}

		p.mu.Lock()
		p.urlMapping[request.CallbackRouting.Url] = p.hostName
		p.mu.Unlock()
	}

	if !found || instance == p.hostName {
		p.logger.Debug("dispatching callback", "instance", p.hostName, "url", request.CallbackRouting.Url)
		dto := sendRequestToDto(request)
		if request.CallbackRouting.Url != "" {
			p.dispatcher.Dispatch(request.CallbackRouting.Url, &CallbackEntry{Token: request.CallbackRouting.Token, Data: dto}, request.CallbackRouting.AllowBatch)
		}
	} else {
		p.logger.Debug("not dispatching callback", "instance", p.hostName, "url", request.CallbackRouting.Url)
	}

	err = msg.Ack()
	if err != nil {
		return errors.Join(ErrAckMessage, err)
	}

	p.logger.Debug("message acked", "instance", p.hostName, "url", request.CallbackRouting.Url)
	return nil
}

func (p *Processor) Start() error {
	p.startSyncURLMapping()

	err := p.mqClient.SubscribeMsg(CallbackTopic, p.handleCallbackMessage)
	if err != nil {
		return fmt.Errorf("failed to subscribe on %s topic: %v", CallbackTopic, err)
	}
	return nil
}

func (p *Processor) startSyncURLMapping() {
	p.waitGroup.Add(1)
	go func() {
		timer := time.NewTicker(syncInterval)
		defer func() {
			timer.Stop()
			p.waitGroup.Done()
		}()
		for {
			mappings, err := p.store.GetURLMappings(p.ctx)
			if err != nil {
				p.logger.Error("failed to get URL mappings", slog.String("err", err.Error()))
				continue
			}

			p.logger.Debug("mapping updated", "mappings", mappings)

			p.mu.Lock()
			p.urlMapping = mappings
			p.mu.Unlock()
			select {
			case <-p.ctx.Done():
				return
			case <-timer.C:
			}
		}
	}()
}

func (p *Processor) GracefulStop() {
	err := p.store.DeleteURLMapping(p.ctx, p.hostName)
	if err != nil {
		p.logger.Error("failed to delete URL mapping", slog.String("err", err.Error()))
	}

	p.cancelAll()

	p.waitGroup.Wait()
}
