package callbacker

import (
	"context"
	"log/slog"
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

	// internal state
	entriesWg sync.WaitGroup
	entries   chan *CallbackEntry

	stop chan struct{}

	sleep      time.Duration
	quarantine *quarantinePolicy

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

func runNewSendManager(u string, c CallbackerI, s store.CallbackerStore, l *slog.Logger, sleep time.Duration, qPolicy *quarantinePolicy) *sendManager {
	m := &sendManager{
		url:        u,
		c:          c,
		s:          s,
		l:          l,
		sleep:      sleep,
		quarantine: qPolicy,

		entries: make(chan *CallbackEntry),
		stop:    make(chan struct{}),
	}

	m.run()
	return m
}

func (m *sendManager) Add(entry *CallbackEntry) {
	m.entriesWg.Add(1) // count the callbacks accepted for processing
	go func() {
		m.entries <- entry
	}()
}

func (m *sendManager) GracefulStop() {
	m.stop <- struct{}{} // signal the `run` goroutine to stop sending callbacks
	m.entriesWg.Wait()   // wait for all accepted callbacks to be consumed

	close(m.entries) // signal the `run` goroutine to exit

	<-m.stop // wait for the `run` goroutine to exit
	close(m.stop)
}

func (m *sendManager) run() {
	m.setMode(ActiveMode)

	go func() {
		var danglingCallbacks []*store.CallbackData
	runLoop:
		for {
			select {
			case callback, ok := <-m.entries:
				if !ok {
					break runLoop
				}

				switch m.getMode() {
				case ActiveMode:
					m.handleActive(callback)
				case QuarantineMode:
					m.handleQuarantine(callback)
				case StoppingMode:
					// add callback to save
					danglingCallbacks = append(danglingCallbacks, toStoreDto(m.url, callback, nil))
				}

				m.entriesWg.Done() // decrease the number of callbacks that need to be processed (send or store on stop)

			case <-m.stop:
				m.setMode(StoppingMode)
			}
		}

		_ = m.s.SetMany(context.Background(), danglingCallbacks)
		m.stop <- struct{}{}
	}()
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

func (m *sendManager) handleActive(callback *CallbackEntry) {
	if m.c.Send(m.url, callback.Token, callback.Data) {
		time.Sleep(m.sleep)
		return
	}

	m.putInQuarantine()
	m.handleQuarantine(callback)
}

func (m *sendManager) handleQuarantine(ce *CallbackEntry) {
	qUntil := m.quarantine.Until(ce.Data.Timestamp)
	err := m.s.Set(context.Background(), toStoreDto(m.url, ce, &qUntil))
	if err != nil {
		m.l.Error("failed to store callback in quarantine", slog.String("url", m.url), slog.String("err", err.Error()))
	}
}

func (m *sendManager) putInQuarantine() {
	m.setMode(QuarantineMode)
	m.l.Warn("send callback failed - putting receiver in quarantine", slog.String("url", m.url), slog.Duration("approx. duration", m.quarantine.baseDuration))

	go func() {
		time.Sleep(m.quarantine.baseDuration)
		m.modeMu.Lock()

		if m.mode != StoppingMode {
			m.mode = ActiveMode
			m.l.Info("receiver is active again after quarantine", slog.String("url", m.url))
		}

		m.modeMu.Unlock()
	}()
}

func toStoreDto(url string, s *CallbackEntry, quntil *time.Time) *store.CallbackData {
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

		QuarantineUntil: quntil,
	}
}
