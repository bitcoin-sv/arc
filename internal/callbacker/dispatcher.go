package callbacker

/* CallbackDispatcher */
/*

The CallbackDispatcher is responsible for routing and dispatching callbacks to appropriate SendManager based on the callback URL.

Key components:
- SenderI Interface: the CallbackDispatcher decorates this interface, enhancing its functionality by managing the actual dispatch logic
- SendManager: each SendManager handles specific types of callbacks, determined by the URL

Dispatch Logic: the CallbackDispatcher ensures that callbacks are sent to the correct SendManager, maintaining efficient processing and delivery.
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

	managers   map[string]*SendManager
	managersMu sync.Mutex

	sendConfig *SendConfig
}

type CallbackEntry struct {
	Token          string
	Data           *Callback
	postponedUntil *time.Time
}

type SendConfig struct {
	Delay                              time.Duration
	PauseAfterSingleModeSuccessfulSend time.Duration
	DelayDuration                      time.Duration
	BatchSendInterval                  time.Duration
	Expiration                         time.Duration
}

func NewCallbackDispatcher(callbacker SenderI, cStore store.CallbackerStore, logger *slog.Logger,
	sendingConfig *SendConfig) *CallbackDispatcher {
	return &CallbackDispatcher{
		sender: callbacker,
		store:  cStore,
		logger: logger.With(slog.String("module", "dispatcher")),

		sendConfig: sendingConfig,
		managers:   make(map[string]*SendManager),
	}
}

func (d *CallbackDispatcher) GetLenMangers() int {
	return len(d.managers)
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
		manager = RunNewSendManager(url, d.sender, d.store, d.logger, d.sendConfig)
		d.managers[url] = manager
	}
	d.managersMu.Unlock()

	manager.Add(dto, allowBatch)
}
