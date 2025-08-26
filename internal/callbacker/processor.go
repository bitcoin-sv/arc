package callbacker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"

	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
	"github.com/bitcoin-sv/arc/internal/callbacker/store"
	"github.com/bitcoin-sv/arc/internal/mq"
)

type Sender interface {
	Send(url, token string, callback *Callback) (success, retry bool)
	SendBatch(url, token string, callbacks []*Callback) (success, retry bool)
}

const (
	batchSizeDefault              = 500
	singleSendDefault             = 5 * time.Second
	expirationDefault             = 24 * time.Hour
	batchSendIntervalDefault      = 5 * time.Second
	storeCallbacksIntervalDefault = 5 * time.Second
	storeCallbackBatchSizeDefault = 20
	sendCallbacksInterval         = 5 * time.Second
)

type Processor struct {
	mqClient               mq.MessageQueueClient
	sender                 Sender
	store                  store.ProcessorStore
	logger                 *slog.Logger
	sendRequestCh          chan *callbacker_api.SendRequest
	storeCallbackBatchSize int
	storeCallbacksInterval time.Duration
	sendCallbacksInterval  time.Duration
	expiration             time.Duration
	batchSize              int
	singleSendInterval     time.Duration
	batchSendInterval      time.Duration
	waitGroup              *sync.WaitGroup
	cancelAll              context.CancelFunc
	ctx                    context.Context
}

func WithSingleSendInterval(d time.Duration) func(*Processor) {
	return func(m *Processor) {
		m.singleSendInterval = d
	}
}

func WithBatchSendInterval(d time.Duration) func(*Processor) {
	return func(m *Processor) {
		m.batchSendInterval = d
	}
}

func WithBatchSize(size int) func(*Processor) {
	return func(m *Processor) {
		m.batchSize = size
	}
}

func WithExpiration(d time.Duration) func(*Processor) {
	return func(m *Processor) {
		m.expiration = d
	}
}

func NewProcessor(sender SenderI, processorStore store.ProcessorStore, mqClient mq.MessageQueueClient, logger *slog.Logger, opts ...func(*Processor)) (*Processor, error) {
	p := &Processor{
		mqClient:               mqClient,
		sender:                 sender,
		store:                  processorStore,
		logger:                 logger,
		sendRequestCh:          make(chan *callbacker_api.SendRequest, 500),
		storeCallbackBatchSize: storeCallbackBatchSizeDefault,
		storeCallbacksInterval: storeCallbacksIntervalDefault,
		sendCallbacksInterval:  sendCallbacksInterval,
		expiration:             expirationDefault,
		batchSize:              batchSizeDefault,
		singleSendInterval:     singleSendDefault,
		batchSendInterval:      batchSendIntervalDefault,
		waitGroup:              &sync.WaitGroup{},
	}
	for _, opt := range opts {
		opt(p)
	}

	ctx, cancelAll := context.WithCancel(context.Background())
	p.cancelAll = cancelAll
	p.ctx = ctx

	return p, nil
}

func (p *Processor) Start() error {
	err := p.mqClient.QueueSubscribe(mq.CallbackTopic, func(msg []byte) error {
		serialized := &callbacker_api.SendRequest{}
		err := proto.Unmarshal(msg, serialized)
		if err != nil {
			return fmt.Errorf("failed to unmarshal send request on %s topic", mq.CallbackTopic)
		}

		p.logger.Info("=== enqueued callback request",
			slog.String("url", serialized.CallbackRouting.Url),
			slog.String("token", serialized.CallbackRouting.Token),
			slog.String("hash", serialized.Txid),
			slog.String("status", serialized.Status.String()),
		)

		p.sendRequestCh <- serialized

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe on %s topic: %v", mq.CallbackTopic, err)
	}
	return nil
}

func toStoreDto(request *callbacker_api.SendRequest) *store.CallbackData {
	return &store.CallbackData{
		URL:          request.CallbackRouting.Url,
		Token:        request.CallbackRouting.Token,
		Timestamp:    request.Timestamp.AsTime(),
		CompetingTxs: request.CompetingTxs,
		TxID:         request.Txid,
		TxStatus:     request.Status.String(),
		ExtraInfo:    ptrTo(request.ExtraInfo),
		MerklePath:   ptrTo(request.MerklePath),
		BlockHash:    ptrTo(request.BlockHash),
		BlockHeight:  ptrTo(request.BlockHeight),
		AllowBatch:   request.CallbackRouting.AllowBatch,
	}
}

type CallbackEntry struct {
	Token      string
	Data       *Callback
	AllowBatch bool
}

func (p *Processor) StartSendCallbacks() {
	ticker := time.NewTicker(p.sendCallbacksInterval)

	p.waitGroup.Add(1)
	go func() {
		defer p.waitGroup.Done()
		for {
			select {
			case <-p.ctx.Done():
				return
			case <-ticker.C:
				callbackRecords, err := p.store.GetUnsent(p.ctx, p.batchSize, p.expiration, false)
				if err != nil {
					p.logger.Error("Failed to get many", slog.String("err", err.Error()))
					continue
				}

				urlCallbacksMap := map[string][]*store.CallbackData{}
				for _, callbackRecord := range callbackRecords {
					urlCallbacksMap[callbackRecord.URL] = append(urlCallbacksMap[callbackRecord.URL], callbackRecord)
				}

				g, _ := errgroup.WithContext(p.ctx)
				g.SetLimit(10)

				for url, callbacks := range urlCallbacksMap {
					g.Go(func() error {
						p.sendCallback(url, callbacks)
						return nil
					})
				}

				err = g.Wait()
				if err != nil {
					p.logger.Error("Failed send callbacks", slog.String("err", err.Error()))
				}
			}
		}
	}()
}

func (p *Processor) sendCallback(url string, cbs []*store.CallbackData) {
	cbIDs := make([]int64, len(cbs))
	for i, cb := range cbs {
		cbIDs[i] = cb.ID
	}
	for _, cb := range cbs {
		cbEntry := toEntry(cb)
		success, retry := p.sender.Send(url, cbEntry.Token, cbEntry.Data)
		if retry || !success {
			err := p.store.UnsetPending(p.ctx, cbIDs)
			if err != nil {
				p.logger.Error("Failed to set not pending", slog.String("err", err.Error()))
			}
			break
		}

		err := p.store.SetSent(p.ctx, []int64{cb.ID})
		if err != nil {
			p.logger.Error("Failed to set sent", slog.String("err", err.Error()))
		}

		time.Sleep(p.singleSendInterval)
	}
}

func (p *Processor) StartSendBatchCallbacks() {
	ticker := time.NewTicker(p.batchSendInterval)

	p.waitGroup.Add(1)
	go func() {
		defer p.waitGroup.Done()
		for {
			select {
			case <-p.ctx.Done():
				return
			case <-ticker.C:
				callbackRecords, err := p.store.GetUnsent(p.ctx, p.batchSize, p.expiration, true)
				if err != nil {
					p.logger.Error("Failed to get many", slog.String("err", err.Error()))
					continue
				}

				urlCallbacksMap := map[string][]*store.CallbackData{}
				for _, callbackRecord := range callbackRecords {
					urlCallbacksMap[callbackRecord.URL] = append(urlCallbacksMap[callbackRecord.URL], callbackRecord)
				}

				g, _ := errgroup.WithContext(p.ctx)
				g.SetLimit(10)

				for url, callbacks := range urlCallbacksMap {
					g.Go(func() error {
						p.sendBatchCallback(url, callbacks)
						return nil
					})
				}

				err = g.Wait()
				if err != nil {
					p.logger.Error("Failed send callbacks", slog.String("err", err.Error()))
				}
			}
		}
	}()
}

func (p *Processor) sendBatchCallback(url string, cbs []*store.CallbackData) {
	batch := make([]*Callback, len(cbs))
	cbIDs := make([]int64, len(cbs))
	for i, cb := range cbs {
		batch[i] = toCallback(cb)
		cbIDs[i] = cb.ID
	}
	success, retry := p.sender.SendBatch(url, cbs[0].Token, batch)
	if retry || !success {
		err := p.store.UnsetPending(p.ctx, cbIDs)
		if err != nil {
			p.logger.Error("Failed to set not pending", slog.String("err", err.Error()))
			return
		}
	}

	err := p.store.SetSent(p.ctx, cbIDs)
	if err != nil {
		p.logger.Error("Failed to set sent", slog.String("err", err.Error()))
	}
}

func toEntry(callbackData *store.CallbackData) CallbackEntry {
	return CallbackEntry{
		Token:      callbackData.Token,
		Data:       toCallback(callbackData),
		AllowBatch: callbackData.AllowBatch,
	}
}

func toCallback(callbackData *store.CallbackData) *Callback {
	return &Callback{
		Timestamp:    callbackData.Timestamp,
		CompetingTxs: callbackData.CompetingTxs,
		TxID:         callbackData.TxID,
		TxStatus:     callbackData.TxStatus,
		ExtraInfo:    callbackData.ExtraInfo,
		MerklePath:   callbackData.MerklePath,
		BlockHash:    callbackData.BlockHash,
		BlockHeight:  callbackData.BlockHeight,
	}
}

func (p *Processor) StartStoreCallbackRequests() {
	ticker := time.NewTicker(p.storeCallbacksInterval)

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
					p.logger.Info("=== Storing callbacks", slog.Int("count", len(toStore)))
					rowsAffected, err := p.store.Insert(p.ctx, toStore)
					if err != nil {
						p.logger.Error("Failed to set many", slog.String("err", err.Error()))
						continue
					}
					p.logger.Info("=== Stored callbacks", slog.Int64("count", rowsAffected))

					toStore = toStore[:0]
				}
			case entry := <-p.sendRequestCh:

				toStore = append(toStore, toStoreDto(entry))

				if len(toStore) >= p.storeCallbackBatchSize {
					p.logger.Info("=== Storing callbacks", slog.Int("count", len(toStore)), slog.Int("batch", p.storeCallbackBatchSize))
					rowsAffected, err := p.store.Insert(p.ctx, toStore)
					if err != nil {
						p.logger.Error("Failed to set many", slog.String("err", err.Error()))
						continue
					}
					p.logger.Info("=== Stored callbacks", slog.Int64("count", rowsAffected))

					toStore = toStore[:0]
				}
			}
		}
	}()
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

				err := p.store.Clear(ctx, olderThan)
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

func (p *Processor) GracefulStop() {
	p.cancelAll()

	p.waitGroup.Wait()
}
