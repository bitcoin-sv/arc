package callbacker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

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
	storeCallbackBatchSizeDefault = 500
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
	clearInterval          time.Duration
	clearRetentionPeriod   time.Duration

	wg        *sync.WaitGroup
	cancelAll context.CancelFunc
	ctx       context.Context
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

func WithClearInterval(d time.Duration) func(*Processor) {
	return func(m *Processor) {
		m.clearInterval = d
	}
}

func WithClearRetentionPeriod(d time.Duration) func(*Processor) {
	return func(m *Processor) {
		m.clearRetentionPeriod = d
	}
}

func WithStoreCallbackBatchSize(d int) func(*Processor) {
	return func(m *Processor) {
		m.storeCallbackBatchSize = d
	}
}

func WithStoreCallbacksInterval(d time.Duration) func(*Processor) {
	return func(m *Processor) {
		m.storeCallbacksInterval = d
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
		wg:                     &sync.WaitGroup{},
	}
	for _, opt := range opts {
		opt(p)
	}

	ctx, cancelAll := context.WithCancel(context.Background())
	p.cancelAll = cancelAll
	p.ctx = ctx

	return p, nil
}

func (p *Processor) Subscribe() error {
	err := p.mqClient.Consume(mq.CallbackTopic, func(msg []byte) error {
		serialized := &callbacker_api.SendRequest{}
		err := proto.Unmarshal(msg, serialized)
		if err != nil {
			return fmt.Errorf("failed to unmarshal send request on %s topic", mq.CallbackTopic)
		}

		p.logger.Debug("Enqueued callback request",
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

func (p *Processor) Start() error {
	err := p.Subscribe()
	if err != nil {
		return err
	}
	p.StartRoutine(p.clearInterval, CallbackStoreCleanup)
	p.StartRoutine(p.sendCallbacksInterval, LoadAndSendSingleCallbacks)
	p.StartRoutine(p.batchSendInterval, LoadAndSendBatchCallbacks)
	p.StartStoreCallbackRequests()

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

func (p *Processor) StartRoutine(tickerInterval time.Duration, routine func(*Processor)) {
	ticker := time.NewTicker(tickerInterval)
	p.wg.Add(1)

	go func() {
		defer func() {
			p.wg.Done()
			ticker.Stop()
		}()

		for {
			select {
			case <-p.ctx.Done():
				return
			case <-ticker.C:
				routine(p)
			}
		}
	}()
}

func (p *Processor) StartStoreCallbackRequests() {
	ticker := time.NewTicker(p.storeCallbacksInterval)

	p.wg.Add(1)
	go func() {
		var toStore []*store.CallbackData
		defer func() {
			p.wg.Done()
			ticker.Stop()
		}()
		for {
			select {
			case <-p.ctx.Done():
				return
			case <-ticker.C:
				if len(toStore) > 0 {
					p.logger.Debug("Storing callbacks", slog.Int("count", len(toStore)))
					rowsAffected, err := p.store.Insert(p.ctx, toStore)
					if err != nil {
						p.logger.Error("Failed to set many", slog.String("err", err.Error()))
						continue
					}
					p.logger.Debug("Stored callbacks", slog.Int64("count", rowsAffected))

					toStore = toStore[:0]
				}
			case entry := <-p.sendRequestCh:
				toStore = append(toStore, toStoreDto(entry))

				if len(toStore) >= p.storeCallbackBatchSize {
					p.logger.Debug("Storing callbacks", slog.Int("count", len(toStore)), slog.Int("batch", p.storeCallbackBatchSize))
					rowsAffected, err := p.store.Insert(p.ctx, toStore)
					if err != nil {
						p.logger.Error("Failed to set many", slog.String("err", err.Error()))
						continue
					}
					p.logger.Debug("Stored callbacks", slog.Int64("count", rowsAffected))

					toStore = toStore[:0]
				}
			}
		}
	}()
}

func (p *Processor) GracefulStop() {
	p.cancelAll()

	p.wg.Wait()
}
