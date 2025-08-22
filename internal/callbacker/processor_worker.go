package callbacker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
	"github.com/bitcoin-sv/arc/internal/callbacker/store"
	"github.com/bitcoin-sv/arc/internal/mq"
)

type ProcessorWorker struct {
	mqClient       mq.MessageQueueClient
	dispatcher     Dispatcher
	store          store.ProcessorStore
	logger         *slog.Logger
	setURLInterval time.Duration
	hostName       string
	waitGroup      *sync.WaitGroup
	cancelAll      context.CancelFunc
	ctx            context.Context

	sendRequestCh          chan *callbacker_api.SendRequest
	storeCallbackBatchSize int

	mu         sync.RWMutex
	urlMapping map[string]string
}

func WithSetURLIntervalWorker(interval time.Duration) func(*Processor) {
	return func(p *Processor) {
		p.setURLInterval = interval
	}
}

func NewProcessorWorker(dispatcher Dispatcher, processorStore store.ProcessorStore, mqClient mq.MessageQueueClient, hostName string, logger *slog.Logger, opts ...func(*ProcessorWorker)) (*ProcessorWorker, error) {
	p := &ProcessorWorker{
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

func (p *ProcessorWorker) Start() error {
	p.startSyncURLMapping()

	err := p.mqClient.QueueSubscribe(mq.CallbackTopic, func(msg []byte) error {
		serialized := &callbacker_api.SendRequest{}
		err := proto.Unmarshal(msg, serialized)
		if err != nil {
			return errors.Join(fmt.Errorf("subscribed on %s topic", mq.CallbackTopic), err)
		}

		p.sendRequestCh <- serialized

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe on %s topic: %v", mq.CallbackTopic, err)
	}
	return nil
}

func toStoreDto(url string, entry CallbackEntry) *store.CallbackData {
	return &store.CallbackData{
		URL:       url,
		Token:     entry.Token,
		Timestamp: entry.Data.Timestamp,

		CompetingTxs: entry.Data.CompetingTxs,
		TxID:         entry.Data.TxID,
		TxStatus:     entry.Data.TxStatus,
		ExtraInfo:    entry.Data.ExtraInfo,
		MerklePath:   entry.Data.MerklePath,

		BlockHash:   entry.Data.BlockHash,
		BlockHeight: entry.Data.BlockHeight,

		AllowBatch: entry.AllowBatch,
	}
}

func (p *ProcessorWorker) StartStoreCallbackRequests() {
	ticker := time.NewTicker(5 * time.Second)

	p.waitGroup.Add(1)
	go func() {
		var toStore []*store.CallbackData
		defer p.waitGroup.Done()
		for {
			select {
			case <-p.ctx.Done():
				return
			case <-ticker.C:
				if len(toStore) > 0 {
					err := p.store.SetMany(p.ctx, toStore)
					if err != nil {
						p.logger.Error("Failed to set many", slog.String("err", err.Error()))
						continue
					}

					toStore = toStore[:0]
				}
			case entry := <-p.sendRequestCh:

				dto := sendRequestToDto(entry)

				storeDto := toStoreDto(entry.CallbackRouting.Url, entry)

				toStore = append(toStore, storeDto)

				if len(toStore) >= p.storeCallbackBatchSize {
					err := p.store.SetMany(p.ctx, toStore)
					if err != nil {
						p.logger.Error("Failed to set many", slog.String("err", err.Error()))
						continue
					}

					toStore = toStore[:0]
				}
			}
		}
	}()
}

func (p *ProcessorWorker) StartCallbackStoreCleanup(interval, olderThanDuration time.Duration) {
	ctx := context.Background()
	ticker := time.NewTicker(interval)

	p.waitGroup.Add(1)
	go func() {
		for {
			defer p.waitGroup.Done()
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
func (p *ProcessorWorker) StartSetUnmappedURLs() {
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

func (p *ProcessorWorker) startSyncURLMapping() {
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

func (p *ProcessorWorker) GracefulStop() {
	rowsAffected, err := p.store.DeleteURLMapping(p.ctx, p.hostName)
	if err != nil {
		p.logger.Error("Failed to delete URL mapping", slog.String("err", err.Error()))
	} else {
		p.logger.Info("Deleted URL mapping", slog.String("hostname", p.hostName), slog.Int64("rows", rowsAffected))
	}

	p.cancelAll()

	p.waitGroup.Wait()
}
