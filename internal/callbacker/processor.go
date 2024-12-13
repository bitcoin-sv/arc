package callbacker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"

	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
	"github.com/bitcoin-sv/arc/internal/callbacker/store"
)

type Processor struct {
	mqClient   MessageQueueClient
	dispatcher *CallbackDispatcher
	store      store.ProcessorStore
	logger     *slog.Logger

	hostName  string
	waitGroup *sync.WaitGroup
	cancelAll context.CancelFunc
	ctx       context.Context

	mu         sync.RWMutex
	urlMapping map[string]string
}

func NewProcessor(dispatcher *CallbackDispatcher, processorStore store.ProcessorStore, mqClient MessageQueueClient, logger *slog.Logger, opts ...func(*Processor)) (*Processor, error) {
	hostName, err := os.Hostname()
	if err != nil {
		return nil, err
	}

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

func sendRequestToDto(r *callbacker_api.SendRequest) *Callback {
	dto := Callback{
		TxID:      r.Txid,
		TxStatus:  r.Status.String(),
		Timestamp: time.Now().UTC(),
	}

	if r.BlockHash != "" {
		dto.BlockHash = ptrTo(r.BlockHash)
		dto.BlockHeight = ptrTo(r.BlockHeight)
	}

	if r.MerklePath != "" {
		dto.MerklePath = ptrTo(r.MerklePath)
	}

	if r.ExtraInfo != "" {
		dto.ExtraInfo = ptrTo(r.ExtraInfo)
	}

	if len(r.CompetingTxs) > 0 {
		dto.CompetingTxs = r.CompetingTxs
	}

	return &dto
}

func (p *Processor) handleCallbackMessage(msg jetstream.Msg) error {
	p.logger.Debug("message received", "instance", p.hostName)

	request := &callbacker_api.SendRequest{}
	err := proto.Unmarshal(msg.Data(), request)
	if err != nil {
		nakErr := msg.Nak()
		if nakErr != nil {
			p.logger.Error("failed to nak message", nakErr)
			return errors.Join(fmt.Errorf("failed to unmarshal message"), nakErr)
		}
		return fmt.Errorf("failed to unmarshal message")
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
					p.logger.Error("failed to ack message", slog.String("err", errAck.Error()))
					return errAck
				}

				return nil
			}

			p.logger.Error("failed to set URL mapping", slog.String("err", err.Error()))

			err = msg.Nak()
			if err != nil {
				p.logger.Error("failed to nak message", slog.String("err", err.Error()))
				return err
			}
			return err
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
		p.logger.Error("failed to ack message", slog.String("err", err.Error()))
	}

	p.logger.Debug("message acked", "instance", p.hostName, "url", request.CallbackRouting.Url)
	return nil
}

func (p *Processor) Start() error {
	err := p.mqClient.SubscribeMsg(CallbackTopic, p.handleCallbackMessage)
	if err != nil {
		return fmt.Errorf("failed to subscribe on %s topic: %v", CallbackTopic, err)
	}

	p.startSyncURLMapping()

	return nil
}

func (p *Processor) startSyncURLMapping() {
	p.waitGroup.Add(1)
	go func() {
		timer := time.NewTimer(time.Second * 5)
		defer func() {
			timer.Stop()
			p.waitGroup.Done()
		}()
		for {
			select {
			case <-p.ctx.Done():
				return
			case <-timer.C:

				mappings, err := p.store.GetURLMappings(p.ctx)
				if err != nil {
					p.logger.Error("failed to get URL mappings", slog.String("err", err.Error()))
					continue
				}

				p.logger.Info("mapping updated", "mappings", mappings)

				p.mu.Lock()
				p.urlMapping = mappings
				p.mu.Unlock()
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
