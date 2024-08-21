package callbacker

import (
	"sync"
	"time"
)

type CallbackDispatcher struct {
	c        CallbackerI
	managers map[string]*sendManager
	mu       sync.Mutex

	sleep time.Duration
}

func NewCallbackDispatcher(callbacker CallbackerI, sleepDuration time.Duration) *CallbackDispatcher {
	return &CallbackDispatcher{
		c:        callbacker,
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

func (d *CallbackDispatcher) dispatch(url, token string, dto *Callback) {
	d.mu.Lock()
	m, ok := d.managers[url]

	if !ok {
		m = runNewSendManager(url, d.c, d.sleep)
		d.managers[url] = m
	}
	d.mu.Unlock()

	m.Add(token, dto)
}

type sendManager struct {
	url string
	c   CallbackerI

	wg sync.WaitGroup
	ch chan *callbackEntry

	sleep time.Duration
}

type callbackEntry struct {
	token string
	data  *Callback
}

func runNewSendManager(u string, c CallbackerI, s time.Duration) *sendManager {
	m := &sendManager{
		url:   u,
		c:     c,
		sleep: s,

		ch: make(chan *callbackEntry),
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
	m.wg.Wait() // wait for all accepted callbacks to be consumed
	close(m.ch) // signal the `run` goroutine to exit
}

func (m *sendManager) run() {
	go func() {
		for {
			select {
			case callback, ok := <-m.ch:
				if !ok {
					return // exit the goroutine when channel is closed
				}

				m.c.Send(m.url, callback.token, callback.data)
				m.wg.Done()

				time.Sleep(m.sleep)
			}
		}
	}()
}
