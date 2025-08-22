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
	"github.com/bitcoin-sv/arc/internal/mq"
)

type Dispatcher interface {
	Dispatch(url string, dto *CallbackEntry)
}

const (
	syncInterval                     = 5 * time.Second
	dispatchPersistedIntervalDefault = 20 * time.Second
)

var (
	ErrUnmarshal  = errors.New("failed to unmarshal message")
	ErrNakMessage = errors.New("failed to nak message")
	ErrAckMessage = errors.New("failed to ack message")
	ErrSetMapping = errors.New("failed to set mapping")
)

type Processor struct {
	mqClient       mq.MessageQueueClient
	dispatcher     Dispatcher
	store          store.ProcessorStore
	logger         *slog.Logger
	setURLInterval time.Duration
	hostName       string
	waitGroup      *sync.WaitGroup
	cancelAll      context.CancelFunc
	ctx            context.Context

	mu         sync.RWMutex
	urlMapping map[string]string
}

func WithSetURLInterval(interval time.Duration) func(*Processor) {
	return func(p *Processor) {
		p.setURLInterval = interval
	}
}

func NewProcessor(dispatcher Dispatcher, processorStore store.ProcessorStore, mqClient mq.MessageQueueClient, hostName string, logger *slog.Logger, opts ...func(*Processor)) (*Processor, error) {
	p := &Processor{
		hostName:       hostName,
		urlMapping:     make(map[string]string),
		dispatcher:     dispatcher,
		waitGroup:      &sync.WaitGroup{},
		store:          processorStore,
		logger:         logger,
		mqClient:       mqClient,
		setURLInterval: dispatchPersistedIntervalDefault,
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
	var errSetURLMapping error
	request := &callbacker_api.SendRequest{}
	err := proto.Unmarshal(msg.Data(), request)
	if err != nil {
		nakErr := msg.Ack() // Ack instead of nak. The same message will always fail to be unmarshalled
		if nakErr != nil {
			return errors.Join(errors.Join(ErrUnmarshal, err), errors.Join(ErrNakMessage, nakErr))
		}
		return errors.Join(ErrUnmarshal, err)
	}

	if request.CallbackRouting.Url == "" {
		p.logger.Warn("Empty URL in callback", slog.String("hash", request.Txid), slog.String("timestamp", request.Timestamp.String()), slog.String("status", request.Status.String()))
		return p.checkAckedMessage(msg, request)
	}

	// check if this processor the first URL of this request is mapped to this instance
	p.mu.Lock()
	instance, found := p.urlMapping[request.CallbackRouting.Url]
	p.mu.Unlock()
	if !found {
		p.logger.Debug("setting URL mapping", "instance", p.hostName, "url", request.CallbackRouting.Url)
		errSetURLMapping = p.store.SetURLMapping(context.Background(), store.URLMapping{
			URL:      request.CallbackRouting.Url,
			Instance: p.hostName,
		})
	}

	if errors.Is(errSetURLMapping, store.ErrURLMappingDuplicateKey) {
		p.logger.Debug("URL already mapped", slog.String("url", request.CallbackRouting.Url), slog.String("err", errSetURLMapping.Error()))

		return p.checkAckedMessage(msg, request)
	}

	if errSetURLMapping != nil {
		p.logger.Error("failed to set URL mapping", slog.String("err", errSetURLMapping.Error()))

		nakErr := msg.Nak()
		if nakErr != nil {
			return errors.Join(errors.Join(ErrSetMapping, err), errors.Join(ErrNakMessage, nakErr))
		}
		return errors.Join(ErrSetMapping, err)
	}

	p.mu.Lock()
	p.urlMapping[request.CallbackRouting.Url] = p.hostName
	p.mu.Unlock()

	if !found || instance == p.hostName {
		p.logger.Debug("dispatching callback", "instance", p.hostName, "url", request.CallbackRouting.Url)
		dto := sendRequestToDto(request)
		if request.CallbackRouting.Url != "" {
			p.dispatcher.Dispatch(request.CallbackRouting.Url, &CallbackEntry{Token: request.CallbackRouting.Token, Data: dto, AllowBatch: request.CallbackRouting.AllowBatch})
		}
	} else {
		p.logger.Debug("not dispatching callback", "instance", p.hostName, "url", request.CallbackRouting.Url)
	}

	return p.checkAckedMessage(msg, request)
}

func (p *Processor) checkAckedMessage(msg jetstream.Msg, request *callbacker_api.SendRequest) error {
	errAck := msg.Ack()
	if errAck != nil {
		return errors.Join(ErrAckMessage, errAck)
	}

	p.logger.Debug("message acked", "instance", p.hostName, "url", request.CallbackRouting.Url)
	return nil
}

func (p *Processor) Start() error {
	err := p.mqClient.ConsumeMsg(mq.CallbackTopic, p.handleCallbackMessage)
	if err != nil {
		return fmt.Errorf("failed to subscribe on %s topic: %v", mq.CallbackTopic, err)
	}
	return nil
}

func (p *Processor) StartCallbackStoreCleanup(interval, olderThanDuration time.Duration) {
	ctx := context.Background()
	ticker := time.NewTicker(interval)

	p.waitGroup.Add(1)
	go func() {
		for {
			select {
			case <-ticker.C:
				n := time.Now()
				midnight := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
				olderThan := midnight.Add(-1 * olderThanDuration)

				err := p.store.DeleteOlderThan(ctx, olderThan)
				if err != nil {
					p.logger.Error("Failed to delete old callbacks in delay", slog.String("err", err.Error()))
				}

			case <-p.ctx.Done():
				p.waitGroup.Done()
				return
			}
		}
	}()
}

// StartSetUnmappedURLs finds unmapped URLs and tries to set them in intervals
func (p *Processor) StartSetUnmappedURLs() {
	ctx := context.Background()

	ticker := time.NewTicker(p.setURLInterval)

	p.waitGroup.Add(1)
	go func() {
		defer func() {
			ticker.Stop()
			p.waitGroup.Done()
		}()

		for {
			select {
			case <-p.ctx.Done():
				return
			case <-ticker.C:
				url, err := p.store.GetUnmappedURL(ctx)
				if err != nil {
					if !errors.Is(err, store.ErrNoUnmappedURLsFound) {
						p.logger.Error("Failed to fetch unmapped url", slog.String("err", err.Error()))
					}
					continue
				}

				err = p.store.SetURLMapping(ctx, store.URLMapping{
					URL:      url,
					Instance: p.hostName,
				})

				if err != nil {
					if errors.Is(err, store.ErrURLMappingDuplicateKey) {
						p.logger.Debug("URL already mapped", slog.String("url", url), slog.String("err", err.Error()))
						continue
					}

					p.logger.Error("Failed to set URL mapping", slog.String("err", err.Error()))
					continue
				}
			}
		}
	}()
}

func (p *Processor) StartSyncURLMapping() {
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
	rowsAffected, err := p.store.DeleteURLMapping(p.ctx, p.hostName)
	if err != nil {
		p.logger.Error("Failed to delete URL mapping", slog.String("err", err.Error()))
	} else {
		p.logger.Info("Deleted URL mapping", slog.String("hostname", p.hostName), slog.Int64("rows", rowsAffected))
	}

	p.cancelAll()

	p.waitGroup.Wait()
}
