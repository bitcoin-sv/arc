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
	"fmt"
	"sync"
)

type CallbackDispatcher struct {
	sender SenderI

	runNewManager func(url string) SendManagerI

	managers   map[string]SendManagerI
	managersMu sync.Mutex
}

type CallbackEntry struct {
	Token      string
	Data       *Callback
	AllowBatch bool
}

func NewCallbackDispatcher(callbacker SenderI,
	runNewManager func(url string) SendManagerI) *CallbackDispatcher {
	return &CallbackDispatcher{
		sender:        callbacker,
		runNewManager: runNewManager,
		managers:      make(map[string]SendManagerI),
	}
}

func (d *CallbackDispatcher) GetLenManagers() int {
	return len(d.managers)
}

func (d *CallbackDispatcher) GracefulStop() {
	fmt.Println("dispatcher shutting down")
	d.managersMu.Lock()
	defer d.managersMu.Unlock()

	for _, manager := range d.managers {
		manager.GracefulStop()
	}

}

func (d *CallbackDispatcher) Dispatch(url string, dto *CallbackEntry) {
	d.managersMu.Lock()
	manager, ok := d.managers[url]

	if !ok {
		manager = d.runNewManager(url)
		d.managers[url] = manager
	}
	manager.Enqueue(*dto)

	d.managersMu.Unlock()
}
