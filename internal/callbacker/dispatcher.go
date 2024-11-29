package callbacker

/* CallbackDispatcher */
/*

The CallbackDispatcher is responsible for routing and dispatching callbacks to appropriate sendManager based on the callback URL.

Key components:
- SenderI Interface: the CallbackDispatcher decorates this interface, enhancing its functionality by managing the actual dispatch logic
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
	sender SenderI
	store  store.CallbackerStore
	logger *slog.Logger

	managers   map[string]*sendManager
	managersMu sync.Mutex

	policy *quarantinePolicy

	sendDelay         time.Duration
	sleep             time.Duration
	batchSendInterval time.Duration
}

type CallbackEntry struct {
	Token          string
	Data           *Callback
	postponedUntil *time.Time
}

type SendConfig struct {
	Delay                              time.Duration
	PauseAfterSingleModeSuccessfulSend time.Duration
	BatchSendInterval                  time.Duration
}

type QuarantineConfig struct {
	BaseDuration, PermQuarantineAfterDuration time.Duration
}

func NewCallbackDispatcher(callbacker SenderI, cStore store.CallbackerStore, logger *slog.Logger,
	sendingConfig *SendConfig, quarantineConfig *QuarantineConfig) *CallbackDispatcher {
	return &CallbackDispatcher{
		sender: callbacker,
		store:  cStore,
		logger: logger.With(slog.String("module", "dispatcher")),

		sendDelay:         sendingConfig.Delay,
		sleep:             sendingConfig.PauseAfterSingleModeSuccessfulSend,
		batchSendInterval: sendingConfig.BatchSendInterval,

		policy: &quarantinePolicy{
			baseDuration:        quarantineConfig.BaseDuration,
			permQuarantineAfter: quarantineConfig.PermQuarantineAfterDuration,
			now:                 time.Now,
		},
		managers: make(map[string]*sendManager),
	}
}

func (d *CallbackDispatcher) GracefulStop() {
	d.managersMu.Lock()
	defer d.managersMu.Unlock()

	for _, manager := range d.managers {
		manager.GracefulStop()
	}
}

func (d *CallbackDispatcher) Dispatch(url string, dto *CallbackEntry, allowBatch bool) {
	d.managersMu.Lock()
	manager, ok := d.managers[url]

	if !ok {
		manager = runNewSendManager(url, d.sender, d.store, d.logger, d.policy, d.sendDelay, d.sleep, d.batchSendInterval)
		d.managers[url] = manager
	}
	d.managersMu.Unlock()

	manager.Add(dto, allowBatch)
}
