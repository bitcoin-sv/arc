package metamorph

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-p2p"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/gocore"
	"github.com/sasha-s/go-deadlock"
)

type processorResponseStatusUpdate struct {
	status      metamorph_api.Status
	source      string
	statusErr   error
	updateStore func() error
	callback    func(err error)
}

type ProcessorResponse struct {
	mu             deadlock.RWMutex
	callerCh       chan StatusAndError
	noStats        bool
	statusUpdateCh chan *processorResponseStatusUpdate

	Hash                  []byte
	Start                 time.Time
	Retries               atomic.Uint32
	Err                   error
	AnnouncedPeers        []p2p.PeerI
	Status                metamorph_api.Status
	LastStatusUpdateNanos atomic.Int64
	Log                   []ProcessorResponseLog
}

func NewProcessorResponse(hash []byte) *ProcessorResponse {
	return newProcessorResponse(hash, metamorph_api.Status_UNKNOWN, nil)
}

// NewProcessorResponseWithStatus creates a new ProcessorResponse with the given status.
// It is used when restoring the ProcessorResponseMap from the database.
func NewProcessorResponseWithStatus(hash []byte, status metamorph_api.Status) *ProcessorResponse {
	return newProcessorResponse(hash, status, nil)
}

func NewProcessorResponseWithChannel(hash []byte, ch chan StatusAndError) *ProcessorResponse {
	return newProcessorResponse(hash, metamorph_api.Status_UNKNOWN, ch)
}

func newProcessorResponse(hash []byte, status metamorph_api.Status, ch chan StatusAndError) *ProcessorResponse {
	pr := &ProcessorResponse{
		Start:          time.Now(),
		Hash:           hash,
		Status:         status,
		callerCh:       ch,
		statusUpdateCh: make(chan *processorResponseStatusUpdate, 10),
		Log: []ProcessorResponseLog{
			{
				DeltaT: 0,
				Status: status.String(),
			},
		},
	}

	go func() {
		for statusUpdate := range pr.statusUpdateCh {
			pr.updateStatus(statusUpdate)
		}
	}()

	return pr
}

func (r *ProcessorResponse) UpdateStatus(statusUpdate *processorResponseStatusUpdate) error {
	r.statusUpdateCh <- statusUpdate
	return nil
}

func (r *ProcessorResponse) updateStatus(statusUpdate *processorResponseStatusUpdate) {
	if r.Status >= statusUpdate.status && statusUpdate.statusErr == nil {
		r.AddLog(statusUpdate.status, statusUpdate.source, "duplicate")
		return
	}

	if err := statusUpdate.updateStore(); err != nil {
		statusUpdate.callback(err)
		return
	}

	var ok bool
	if statusUpdate.statusErr != nil {
		ok = r.setStatusAndErrorInternal(statusUpdate.status, statusUpdate.statusErr, statusUpdate.source)
	} else {
		ok = r.setStatusInternal(statusUpdate.status, statusUpdate.source)
	}

	statKey := fmt.Sprintf("%d: %s", statusUpdate.status, statusUpdate.status.String())
	r.LastStatusUpdateNanos.Store(gocore.NewStat("processor - async").NewStat(statKey).AddTime(r.LastStatusUpdateNanos.Load()))

	if !ok {
		statusUpdate.callback(fmt.Errorf("failed to send status update to caller"))
		return
	}

	if !r.noStats {
		statusUpdate.callback(nil)
	}
}

func (r *ProcessorResponse) SetPeers(peers []p2p.PeerI) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	r.AnnouncedPeers = peers
}

func (r *ProcessorResponse) GetPeers() []p2p.PeerI {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.AnnouncedPeers
}

func (r *ProcessorResponse) String() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.Err != nil {
		return fmt.Sprintf("%x: %s [%s] %s", bt.ReverseBytes(r.Hash), r.Start.Format(time.RFC3339), r.Status.String(), r.Err.Error())
	}
	return fmt.Sprintf("%x: %s [%s]", bt.ReverseBytes(r.Hash), r.Start.Format(time.RFC3339), r.Status.String())
}

func (r *ProcessorResponse) setStatus(status metamorph_api.Status, source string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.setStatusInternal(status, source)
}

func (r *ProcessorResponse) setStatusInternal(status metamorph_api.Status, source string) bool {
	r.Status = status

	sae := StatusAndError{
		Hash:   r.Hash,
		Status: r.Status,
	}

	ch := r.callerCh

	r.AddLog(status, source, "")

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

	r.AddLog(r.Status, source, err.Error())

	r.mu.Unlock()

	if ch != nil {
		return utils.SafeSend(ch, sae)
	}

	return true
}

func (r *ProcessorResponse) setStatusAndError(status metamorph_api.Status, err error, source string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.setStatusAndErrorInternal(status, err, source)
}

func (r *ProcessorResponse) setStatusAndErrorInternal(status metamorph_api.Status, err error, source string) bool {
	r.Status = status
	r.Err = err

	sae := StatusAndError{
		Hash:   r.Hash,
		Status: r.Status,
		Err:    err,
	}

	ch := r.callerCh

	r.AddLog(status, source, err.Error())

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

func (r *ProcessorResponse) AddLog(status metamorph_api.Status, source string, info string) {
	r.Log = append(r.Log, ProcessorResponseLog{
		DeltaT: time.Since(r.Start).Nanoseconds(),
		Status: status.String(),
		Source: source,
		Info:   info,
	})
}
