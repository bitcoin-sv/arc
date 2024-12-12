package callbacker

/* SendManager */
/*

The SendManager is responsible for managing the sequential sending of callbacks to a specified URL.
It supports single and batched callbacks, handles failures by placing the URL in failed state, and ensures
safe storage of unsent callbacks during graceful shutdowns.
The manager operates in various modes:
	- ActiveMode (normal sending)
	- StoppingMode (for graceful shutdown).

It processes callbacks from two channels, ensuring either single or batch dispatch, and manages retries based on a failure policy.

Key components:
- SenderI : responsible for sending callbacks

Sending logic: callbacks are sent to the designated URL one at a time, ensuring sequential and orderly processing.

Failure handling: if a URL fails to respond with a success status, the URL is placed in failed state (based on a defined policy).
	During this period, all callbacks for the failed URL are stored with a failed timestamp, preventing further dispatch attempts until the we retry again.

Graceful Shutdown: on service termination, the SendManager ensures that any unsent callbacks are safely persisted in the store, ensuring no loss of data during shutdown.

*/

import (
	"context"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/bitcoin-sv/arc/internal/callbacker/store"
)

type SendManager struct {
	url string

	// dependencies
	sender SenderI
	store  store.CallbackerStore
	logger *slog.Logger

	expiration time.Duration

	// internal state
	entries      chan *CallbackEntry
	batchEntries chan *CallbackEntry

	stop chan struct{}

	sendDelay         time.Duration
	singleSendSleep   time.Duration
	batchSendInterval time.Duration
	delayDuration     time.Duration

	modeMu sync.Mutex
	mode   mode
}

type mode uint8

var infinity = time.Date(2999, time.January, 1, 0, 0, 0, 0, time.UTC)

const (
	IdleMode mode = iota
	ActiveMode
	StoppingMode

	entriesBufferSize = 10000
)

func WithBufferSize(size int) func(*SendManager) {
	return func(m *SendManager) {
		m.entries = make(chan *CallbackEntry, size)
		m.batchEntries = make(chan *CallbackEntry, size)
	}
}

func RunNewSendManager(url string, sender SenderI, store store.CallbackerStore, logger *slog.Logger, sendingConfig *SendConfig, opts ...func(*SendManager)) *SendManager {
	const defaultBatchSendInterval = 5 * time.Second

	batchSendInterval := defaultBatchSendInterval
	if sendingConfig.BatchSendInterval != 0 {
		batchSendInterval = sendingConfig.BatchSendInterval
	}

	m := &SendManager{
		url:    url,
		sender: sender,
		store:  store,
		logger: logger,

		sendDelay:         sendingConfig.Delay,
		singleSendSleep:   sendingConfig.PauseAfterSingleModeSuccessfulSend,
		batchSendInterval: batchSendInterval,
		delayDuration:     sendingConfig.DelayDuration,
		expiration:        sendingConfig.Expiration,

		entries:      make(chan *CallbackEntry, entriesBufferSize),
		batchEntries: make(chan *CallbackEntry, entriesBufferSize),
		stop:         make(chan struct{}),
	}

	for _, opt := range opts {
		opt(m)
	}

	m.run()
	return m
}

func (m *SendManager) Add(entry *CallbackEntry, batch bool) {
	if batch {
		select {
		case m.batchEntries <- entry:
		default:
			m.logger.Warn("Batch entry buffer is full - storing entry on DB",
				slog.String("url", m.url),
				slog.String("token", entry.Token),
				slog.String("hash", entry.Data.TxID),
			)
			m.storeToDB(entry, ptrTo(time.Now()))
		}
		return
	}

	select {
	case m.entries <- entry:
	default:
		m.logger.Warn("Single entry buffer is full - storing entry on DB",
			slog.String("url", m.url),
			slog.String("token", entry.Token),
			slog.String("hash", entry.Data.TxID),
		)
		m.storeToDB(entry, ptrTo(time.Now()))
	}
}

func (m *SendManager) storeToDB(entry *CallbackEntry, postponeUntil *time.Time) {
	if entry == nil {
		return
	}
	callbackData := toStoreDto(m.url, entry, postponeUntil, false)
	err := m.store.Set(context.Background(), callbackData)
	if err != nil {
		m.logger.Error("Failed to set callback data", slog.String("hash", callbackData.TxID), slog.String("status", callbackData.TxStatus), slog.String("err", err.Error()))
	}
}

func (m *SendManager) GracefulStop() {
	m.setMode(StoppingMode) // signal the `run` goroutine to stop sending callbacks

	// signal the `run` goroutine to exit
	close(m.entries)
	close(m.batchEntries)

	<-m.stop // wait for the `run` goroutine to exit
	close(m.stop)
}

func (m *SendManager) run() {
	m.setMode(ActiveMode)

	go func() {
		var danglingCallbacks []*store.CallbackData
		var danglingBatchedCallbacks []*store.CallbackData

		var runWg sync.WaitGroup
		runWg.Add(2)

		go func() {
			defer runWg.Done()
			danglingCallbacks = m.consumeSingleCallbacks()
		}()

		go func() {
			defer runWg.Done()
			danglingBatchedCallbacks = m.consumeBatchedCallbacks()
		}()

		runWg.Wait()

		totalCallbacks := append(danglingCallbacks, danglingBatchedCallbacks...)
		m.logger.Info("Storing pending callbacks", slog.Int("count", len(totalCallbacks)))

		// store unsent callbacks
		err := m.store.SetMany(context.Background(), totalCallbacks)
		if err != nil {
			m.logger.Error("Failed to store pending callbacks", slog.Int("count", len(totalCallbacks)))
		}
		m.stop <- struct{}{}
	}()
}

func (m *SendManager) consumeSingleCallbacks() []*store.CallbackData {
	var danglingCallbacks []*store.CallbackData

	for {
		callback, ok := <-m.entries
		if !ok {
			break
		}

		switch m.getMode() {
		case ActiveMode:
			m.send(callback)
		case StoppingMode:
			// add callback to save
			danglingCallbacks = append(danglingCallbacks, toStoreDto(m.url, callback, nil, false))
		}
	}

	return danglingCallbacks
}

func (m *SendManager) consumeBatchedCallbacks() []*store.CallbackData {
	const batchSize = 50
	var danglingCallbacks []*store.CallbackData

	var callbacks []*CallbackEntry
	sendInterval := time.NewTicker(m.batchSendInterval)

runLoop:
	for {
		select {
		// put callback to process
		case callback, ok := <-m.batchEntries:
			if !ok {
				break runLoop
			}
			callbacks = append(callbacks, callback)

		// process batch
		case <-sendInterval.C:
			if len(callbacks) == 0 {
				continue
			}

			switch m.getMode() {
			case ActiveMode:
				// send batch
				n := int(math.Min(float64(len(callbacks)), batchSize))
				batch := callbacks[:n] // get n callbacks to send

				m.sendBatch(batch)
				callbacks = callbacks[n:] // shrink slice

			case StoppingMode:
				// add callback to save
				danglingCallbacks = append(danglingCallbacks, toStoreDtoCollection(m.url, nil, true, callbacks)...)
				callbacks = nil
			}

			sendInterval.Reset(m.batchSendInterval)
		}
	}

	if len(callbacks) > 0 {
		// add callback to save
		danglingCallbacks = append(danglingCallbacks, toStoreDtoCollection(m.url, nil, true, callbacks)...)
	}

	return danglingCallbacks
}

func (m *SendManager) getMode() mode {
	m.modeMu.Lock()
	defer m.modeMu.Unlock()

	return m.mode
}

func (m *SendManager) setMode(v mode) {
	m.modeMu.Lock()
	m.mode = v
	m.modeMu.Unlock()
}

func (m *SendManager) send(callback *CallbackEntry) {
	// quick fix for client issue with to fast callbacks
	time.Sleep(m.sendDelay)

	success, retry := m.sender.Send(m.url, callback.Token, callback.Data)
	if !retry || success {
		time.Sleep(m.singleSendSleep)
		return
	}

	until := time.Now().Add(m.delayDuration)
	if time.Since(callback.Data.Timestamp) > m.expiration {
		until = infinity
	}

	err := m.store.Set(context.Background(), toStoreDto(m.url, callback, &until, false))
	if err != nil {
		m.logger.Error("failed to store failed callback in db", slog.String("url", m.url), slog.String("err", err.Error()))
	}
}

func (m *SendManager) sendBatch(batch []*CallbackEntry) {
	token := batch[0].Token
	callbacks := make([]*Callback, len(batch))
	for i, e := range batch {
		callbacks[i] = e.Data
	}

	// quick fix for client issue with to fast callbacks
	time.Sleep(m.sendDelay)

	success, retry := m.sender.SendBatch(m.url, token, callbacks)
	if !retry || success {
		return
	}

	until := time.Now().Add(m.delayDuration)
	if time.Since(batch[0].Data.Timestamp) > m.expiration {
		until = infinity
	}
	err := m.store.SetMany(context.Background(), toStoreDtoCollection(m.url, &until, true, batch))
	if err != nil {
		m.logger.Error("failed to store failed callbacks in db", slog.String("url", m.url), slog.String("err", err.Error()))
	}
}

func toStoreDto(url string, entry *CallbackEntry, postponedUntil *time.Time, allowBatch bool) *store.CallbackData {
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

		PostponedUntil: postponedUntil,
		AllowBatch:     allowBatch,
	}
}

func toStoreDtoCollection(url string, postponedUntil *time.Time, allowBatch bool, entries []*CallbackEntry) []*store.CallbackData {
	res := make([]*store.CallbackData, len(entries))
	for i, e := range entries {
		res[i] = toStoreDto(url, e, postponedUntil, allowBatch)
	}

	return res
}
