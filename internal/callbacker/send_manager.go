package callbacker

/* sendManager */
/*

The SendManager is responsible for managing the sending of callbacks to a specific URL in a sequential (serial) manner.
It ensures callbacks are sent efficiently while adhering to policies regarding failed deliveries.

Key components:
- CallbackerI : responsible for sending callbacks
- quarantine policy: the duration for quarantining a URL are governed by a configurable policy, determining how long the URL remains inactive before retry attempts

Sending logic: callbacks are sent to the designated URL one at a time, ensuring sequential and orderly processing.

Quarantine handling: if a URL fails to respond with a success status, the URL is placed in quarantine (based on a defined policy).
	During this period, all callbacks for the quarantined URL are stored with a quarantine timestamp, preventing further dispatch attempts until the quarantine expires.

Graceful Shutdown: on service termination, the sendManager ensures that any unsent callbacks are safely persisted in the store, ensuring no loss of data during shutdown.

*/

import (
	"context"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/bitcoin-sv/arc/internal/callbacker/store"
)

type sendManager struct {
	url string

	// dependencies
	c CallbackerI
	s store.CallbackerStore
	l *slog.Logger
	q *quarantinePolicy

	// internal state
	entriesWg    sync.WaitGroup
	entries      chan *CallbackEntry
	batchEntries chan *CallbackEntry

	stop chan struct{}

	singleSendSleep   time.Duration
	batchSendInterval time.Duration

	modeMu sync.Mutex
	mode   mode
}

type mode uint8

const (
	IdleMode mode = iota
	ActiveMode
	QuarantineMode
	StoppingMode
)

func runNewSendManager(u string, c CallbackerI, s store.CallbackerStore, l *slog.Logger, q *quarantinePolicy,
	singleSendSleep, batchSendInterval time.Duration) *sendManager {

	const defaultBatchSendInterval = time.Duration(5 * time.Second)
	if batchSendInterval == 0 {
		batchSendInterval = defaultBatchSendInterval
	}

	m := &sendManager{
		url: u,
		c:   c,
		s:   s,
		l:   l,
		q:   q,

		singleSendSleep:   singleSendSleep,
		batchSendInterval: batchSendInterval,

		entries:      make(chan *CallbackEntry),
		batchEntries: make(chan *CallbackEntry),
		stop:         make(chan struct{}),
	}

	m.run()
	return m
}

func (m *sendManager) Add(entry *CallbackEntry, batch bool) {
	m.entriesWg.Add(1) // count the callbacks accepted for processing
	go func() {
		defer m.entriesWg.Done()

		if batch {
			m.batchEntries <- entry
		} else {
			m.entries <- entry
		}
	}()
}

func (m *sendManager) GracefulStop() {
	m.setMode(StoppingMode) // signal the `run` goroutine to stop sending callbacks
	m.entriesWg.Wait()      // wait for all accepted callbacks to be consumed

	// signal the `run` goroutine to exit
	close(m.entries)
	close(m.batchEntries)

	<-m.stop // wait for the `run` goroutine to exit
	close(m.stop)
}

func (m *sendManager) run() {
	m.setMode(ActiveMode)

	go func() {
		var danglingCallbacks []*store.CallbackData
		var dalglingBatchedCallbacks []*store.CallbackData

		var runWg sync.WaitGroup
		runWg.Add(2)

		go func() {
			defer runWg.Done()
			danglingCallbacks = m.consumeSingleCallbacks()
		}()

		go func() {
			defer runWg.Done()
			dalglingBatchedCallbacks = m.consumeBatchedCallbacks()
		}()
		runWg.Wait()

		// store unsent callbacks
		_ = m.s.SetMany(context.Background(), append(danglingCallbacks, dalglingBatchedCallbacks...))
		m.stop <- struct{}{}
	}()
}

func (m *sendManager) consumeSingleCallbacks() []*store.CallbackData {
	var danglingCallbacks []*store.CallbackData

	for {
		callback, ok := <-m.entries
		if !ok {
			break
		}

		switch m.getMode() {
		case ActiveMode:
			m.send(callback)
		case QuarantineMode:
			m.handleQuarantine(callback)
		case StoppingMode:
			// add callback to save
			danglingCallbacks = append(danglingCallbacks, toStoreDto(m.url, callback, nil, false))
		}
	}

	return danglingCallbacks
}

func (m *sendManager) consumeBatchedCallbacks() []*store.CallbackData {
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

			case QuarantineMode:
				m.handleQuarantineBatch(callbacks)
				callbacks = nil

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

func (m *sendManager) getMode() mode {
	m.modeMu.Lock()
	defer m.modeMu.Unlock()

	return m.mode
}

func (m *sendManager) setMode(v mode) {
	m.modeMu.Lock()
	m.mode = v
	m.modeMu.Unlock()
}

func (m *sendManager) send(callback *CallbackEntry) {
	if m.c.Send(m.url, callback.Token, callback.Data) {
		time.Sleep(m.singleSendSleep)
		return
	}

	m.putInQuarantine()
	m.handleQuarantine(callback)
}

func (m *sendManager) handleQuarantine(ce *CallbackEntry) {
	qUntil := m.q.Until(ce.Data.Timestamp)
	err := m.s.Set(context.Background(), toStoreDto(m.url, ce, &qUntil, false))
	if err != nil {
		m.l.Error("failed to store callback in quarantine", slog.String("url", m.url), slog.String("err", err.Error()))
	}
}

func (m *sendManager) sendBatch(batch []*CallbackEntry) {
	token := batch[0].Token
	callbacks := make([]*Callback, len(batch))
	for i, e := range batch {
		callbacks[i] = e.Data
	}

	if m.c.SendBatch(m.url, token, callbacks) {
		return
	}

	m.putInQuarantine()
	m.handleQuarantineBatch(batch)
}

func (m *sendManager) handleQuarantineBatch(batch []*CallbackEntry) {
	qUntil := m.q.Until(batch[0].Data.Timestamp)
	err := m.s.SetMany(context.Background(), toStoreDtoCollection(m.url, &qUntil, true, batch))
	if err != nil {
		m.l.Error("failed to store callbacks in quarantine", slog.String("url", m.url), slog.String("err", err.Error()))
	}
}

func (m *sendManager) putInQuarantine() {
	m.setMode(QuarantineMode)
	m.l.Warn("send callback failed - putting receiver in quarantine", slog.String("url", m.url), slog.Duration("approx. duration", m.q.baseDuration))

	go func() {
		time.Sleep(m.q.baseDuration)
		m.modeMu.Lock()

		if m.mode != StoppingMode {
			m.mode = ActiveMode
			m.l.Info("receiver is active again after quarantine", slog.String("url", m.url))
		}

		m.modeMu.Unlock()
	}()
}

func toStoreDto(url string, s *CallbackEntry, postponedUntil *time.Time, allowBatch bool) *store.CallbackData {
	return &store.CallbackData{
		Url:       url,
		Token:     s.Token,
		Timestamp: s.Data.Timestamp,

		CompetingTxs: s.Data.CompetingTxs,
		TxID:         s.Data.TxID,
		TxStatus:     s.Data.TxStatus,
		ExtraInfo:    s.Data.ExtraInfo,
		MerklePath:   s.Data.MerklePath,

		BlockHash:   s.Data.BlockHash,
		BlockHeight: s.Data.BlockHeight,

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
