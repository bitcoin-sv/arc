package processor_response

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/gocore"
	"github.com/sasha-s/go-deadlock"
)

type StatusAndError struct {
	Hash   *chainhash.Hash
	Status metamorph_api.Status
	Err    error
}

type ProcessorResponseStatusUpdate struct {
	Status         metamorph_api.Status
	StatusErr      error
	UpdateStore    func() error
	IgnoreCallback bool
	Callback       func(err error)
}

type ProcessorResponse struct {
	callerCh              chan StatusAndError
	NoStats               bool `json:"noStats"`
	statusUpdateCh        chan *ProcessorResponseStatusUpdate
	Hash                  *chainhash.Hash `json:"hash"`
	Start                 time.Time       `json:"start"`
	Retries               atomic.Uint32   `json:"retries"`
	LastStatusUpdateNanos atomic.Int64    `json:"lastStatusUpdateNanos"`
	// The following fields are protected by the mutex
	mu     deadlock.RWMutex
	Err    error                `json:"err"`
	Status metamorph_api.Status `json:"status"`
}

func NewProcessorResponse(hash *chainhash.Hash) *ProcessorResponse {
	return newProcessorResponse(hash, metamorph_api.Status_RECEIVED, nil)
}

// NewProcessorResponseWithStatus creates a new ProcessorResponse with the given status.
// It is used when restoring the ProcessorResponseMap from the database.
func NewProcessorResponseWithStatus(hash *chainhash.Hash, status metamorph_api.Status) *ProcessorResponse {
	return newProcessorResponse(hash, status, nil)
}

func NewProcessorResponseWithChannel(hash *chainhash.Hash, ch chan StatusAndError) *ProcessorResponse {
	return newProcessorResponse(hash, metamorph_api.Status_RECEIVED, ch)
}

func newProcessorResponse(hash *chainhash.Hash, status metamorph_api.Status, ch chan StatusAndError) *ProcessorResponse {
	pr := &ProcessorResponse{
		Start:          time.Now(),
		Hash:           hash,
		Status:         status,
		callerCh:       ch,
		statusUpdateCh: make(chan *ProcessorResponseStatusUpdate, 10),
	}
	pr.LastStatusUpdateNanos.Store(pr.Start.UnixNano())

	go func() {
		for statusUpdate := range pr.statusUpdateCh {
			pr.updateStatus(statusUpdate)
		}
	}()

	return pr
}

func (r *ProcessorResponse) Close() {
	defer func() {
		_ = recover()
	}()

	if r.statusUpdateCh != nil {
		close(r.statusUpdateCh)
	}
}

func (r *ProcessorResponse) UpdateStatus(statusUpdate *ProcessorResponseStatusUpdate) {
	r.statusUpdateCh <- statusUpdate
}

func (r *ProcessorResponse) updateStatus(statusUpdate *ProcessorResponseStatusUpdate) {
	// If this transaction has already been mined, ignore any further updates
	if r.Status == metamorph_api.Status_MINED {
		return
	}

	if statusUpdate.UpdateStore != nil {
		if err := statusUpdate.UpdateStore(); err != nil {
			r.setErr(err, "processorResponse")
			statusUpdate.Callback(err)
			return
		}
	}

	if statusUpdate.StatusErr != nil {
		_ = r.setStatusAndError(statusUpdate.Status, statusUpdate.StatusErr)
	} else {
		_ = r.setStatus(statusUpdate.Status)
	}

	statKey := fmt.Sprintf("%d: %s", statusUpdate.Status, statusUpdate.Status.String())
	r.LastStatusUpdateNanos.Store(gocore.NewStat("processorResponse").NewStat(statKey).AddTime(r.LastStatusUpdateNanos.Load()))

	if !statusUpdate.IgnoreCallback {
		statusUpdate.Callback(nil)
	}
}

func (r *ProcessorResponse) String() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.Err != nil {
		return fmt.Sprintf("%v: %s [%s] %s", r.Hash, r.Start.Format(time.RFC3339), r.Status.String(), r.Err.Error())
	}
	return fmt.Sprintf("%v: %s [%s]", r.Hash, r.Start.Format(time.RFC3339), r.Status.String())
}

func (r *ProcessorResponse) setStatus(status metamorph_api.Status) bool {
	r.mu.Lock()
	r.Status = status
	r.mu.Unlock()

	sae := StatusAndError{
		Hash:   r.Hash,
		Status: r.Status,
	}

	ch := r.callerCh

	if ch != nil {
		return utils.SafeSend(ch, sae)
	}

	return true
}

func (r *ProcessorResponse) GetStatus() metamorph_api.Status {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.Status
}

func (r *ProcessorResponse) setErr(err error, source string) bool {
	r.mu.Lock()

	r.Err = err

	sae := StatusAndError{
		Hash:   r.Hash,
		Status: r.Status,
		Err:    err,
	}

	ch := r.callerCh

	r.mu.Unlock()

	if ch != nil {
		return utils.SafeSend(ch, sae)
	}

	return true
}

func (r *ProcessorResponse) setStatusAndError(status metamorph_api.Status, err error) bool {
	r.Status = status
	r.Err = err

	sae := StatusAndError{
		Hash:   r.Hash,
		Status: r.Status,
		Err:    err,
	}

	ch := r.callerCh

	if ch != nil {
		return utils.SafeSend(ch, sae)
	}

	return true
}

func (r *ProcessorResponse) GetErr() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.Err
}

func (r *ProcessorResponse) GetRetries() uint32 {
	return r.Retries.Load()
}

func (r *ProcessorResponse) IncrementRetry() uint32 {
	r.Retries.Add(1)
	return r.Retries.Load()
}
