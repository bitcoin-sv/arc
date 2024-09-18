package callbacker

/* CallbackDispatcher */
/*

The CallbackDispatcher is a decorator of the CallbackerI interface, responsible for routing and dispatching callbacks to appropriate sendManager based on the callback URL.

Key components:
- CallbackerI Interface: the CallbackDispatcher decorates this interface, enhancing its functionality by managing the actual dispatch logic
- sendManager: each sendManager handles specific types of callbacks, determined by the URL

Dispatch Logic: the CallbackDispatcher ensures that callbacks are sent to the correct sendManager, maintaining efficient processing and delivery.
Graceful Shutdown: on service termination, the CallbackDispatcher ensures all active sendManagers are gracefully stopped, allowing in-progress callbacks to complete and safely shutting down the dispatch process.

*/

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
	Token          string
	Data           *Callback
	postponedUntil *time.Time
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
