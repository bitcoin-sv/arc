package callbacker

import (
	"log/slog"
	"sync"
	"time"

	"github.com/bitcoin-sv/arc/internal/callbacker/store"
)

type CallbackDispatcher struct {
	c CallbackerI
	s store.CallbackerStore
	l *slog.Logger

	managers   map[string]*sendManager
	managersMu sync.Mutex

	sleep  time.Duration
	policy *quarantinePolicy
}

type CallbackEntry struct {
	Token           string
	Data            *Callback
	quarantineUntil *time.Time
}

func NewCallbackDispatcher(callbacker CallbackerI, store store.CallbackerStore, logger *slog.Logger,
	sleepDuration, quarantineBaseDuration, permQuarantineAfterDuration time.Duration) *CallbackDispatcher {

	return &CallbackDispatcher{
		c:     callbacker,
		s:     store,
		l:     logger.With(slog.String("module", "dispatcher")),
		sleep: sleepDuration,
		policy: &quarantinePolicy{
			baseDuration:        quarantineBaseDuration,
			permQuarantineAfter: permQuarantineAfterDuration,
			now:                 time.Now,
		},
		managers: make(map[string]*sendManager),
	}
}

func (d *CallbackDispatcher) Send(url, token string, dto *Callback) bool {
	d.Dispatch(url, &CallbackEntry{Token: token, Data: dto})
	return true
}

func (d *CallbackDispatcher) Health() error {
	return d.c.Health()
}

func (d *CallbackDispatcher) GracefulStop() {
	d.managersMu.Lock()
	defer d.managersMu.Unlock()

	for _, m := range d.managers {
		m.GracefulStop()
	}
}

func (d *CallbackDispatcher) Dispatch(url string, dto *CallbackEntry) {
	d.managersMu.Lock()
	m, ok := d.managers[url]

	if !ok {
		m = runNewSendManager(url, d.c, d.s, d.l, d.sleep, d.policy)
		d.managers[url] = m
	}
	d.managersMu.Unlock()

	m.Add(dto)
}
