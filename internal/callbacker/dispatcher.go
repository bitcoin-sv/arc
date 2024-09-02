package callbacker

import (
	"context"
	"sync"
	"time"

	"github.com/bitcoin-sv/arc/internal/callbacker/store"
)

type CallbackDispatcher struct {
	c CallbackerI
	s store.CallbackerStore

	managers map[string]*sendManager
	mu       sync.Mutex

	sleep time.Duration
}

func NewCallbackDispatcher(callbacker CallbackerI, store store.CallbackerStore, sleepDuration time.Duration) *CallbackDispatcher {
	return &CallbackDispatcher{
		c:        callbacker,
		s:        store,
		sleep:    sleepDuration,
		managers: make(map[string]*sendManager),
	}
}

func (d *CallbackDispatcher) Send(url, token string, dto *Callback) {
	d.dispatch(url, token, dto)
}

func (d *CallbackDispatcher) Health() error {
	return d.c.Health()
}

func (d *CallbackDispatcher) GracefulStop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, m := range d.managers {
		m.GracefulStop()
	}
}

func (d *CallbackDispatcher) Init() error {
	const bundleSize = 100
	ctx := context.Background()

	for {
		callbacks, err := d.s.PopMany(ctx, bundleSize)
		if err != nil || len(callbacks) > 0 {
			return err
		}

		for _, c := range callbacks {
			d.dispatch(c.Url, c.Token, toCallback(c))
		}
	}
}

func (d *CallbackDispatcher) dispatch(url, token string, dto *Callback) {
	d.mu.Lock()
	m, ok := d.managers[url]

	if !ok {
		m = runNewSendManager(url, d.c, d.s, d.sleep)
		d.managers[url] = m
	}
	d.mu.Unlock()

	m.Add(token, dto)
}

type sendManager struct {
	url string
	c   CallbackerI
	s   store.CallbackerStore

	wg       sync.WaitGroup
	ch       chan *callbackEntry
	stop     chan struct{}
	stopping bool

	sleep time.Duration
}

type callbackEntry struct {
	token string
	data  *Callback
}

func runNewSendManager(u string, c CallbackerI, s store.CallbackerStore, slp time.Duration) *sendManager {
	m := &sendManager{
		url:   u,
		c:     c,
		s:     s,
		sleep: slp,

		ch:   make(chan *callbackEntry),
		stop: make(chan struct{}),
	}

	m.run()
	return m
}

func (m *sendManager) Add(token string, dto *Callback) {
	m.wg.Add(1)
	go func() {
		m.ch <- &callbackEntry{token: token, data: dto}
	}()
}

func (m *sendManager) GracefulStop() {
	m.stop <- struct{}{} // signal the `run` goroutine to stop processing
	m.wg.Wait()          // wait for all accepted callbacks to be consumed

	close(m.ch) // signal the `run` goroutine to exit

	<-m.stop // wait for the `run` goroutine to exit
	close(m.stop)
}

func (m *sendManager) run() {
	go func() {
		var danglingCallbacks []*store.CallbackData

	handleCallbacks:
		for {
			select {
			case callback, ok := <-m.ch:
				if !ok {
					break handleCallbacks
				}

				if m.stopping {
					// add callback to save
					danglingCallbacks = append(danglingCallbacks, toStoreDto(m.url, callback))
				} else {
					m.c.Send(m.url, callback.token, callback.data)
					time.Sleep(m.sleep)
				}

				m.wg.Done()

			case <-m.stop:
				m.stopping = true
			}
		}

		_ = m.s.SetMany(context.Background(), danglingCallbacks)
		m.stop <- struct{}{}
	}()
}

func toStoreDto(url string, s *callbackEntry) *store.CallbackData {
	return &store.CallbackData{
		Url:       url,
		Token:     s.token,
		Timestamp: s.data.Timestamp,

		CompetingTxs: s.data.CompetingTxs,
		TxID:         s.data.TxID,
		TxStatus:     s.data.TxStatus,
		ExtraInfo:    s.data.ExtraInfo,
		MerklePath:   s.data.MerklePath,

		BlockHash:   s.data.BlockHash,
		BlockHeight: s.data.BlockHeight,
	}
}

func toCallback(dto *store.CallbackData) *Callback {
	return &Callback{
		Timestamp: dto.Timestamp,

		CompetingTxs: dto.CompetingTxs,
		TxID:         dto.TxID,
		TxStatus:     dto.TxStatus,
		ExtraInfo:    dto.ExtraInfo,
		MerklePath:   dto.MerklePath,

		BlockHash:   dto.BlockHash,
		BlockHeight: dto.BlockHeight,
	}
}
