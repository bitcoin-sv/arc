package metamorph

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-p2p"
	"github.com/ordishs/go-utils"
	"github.com/sasha-s/go-deadlock"
)

type ProcessorResponse struct {
	mu                    deadlock.RWMutex
	ch                    chan StatusAndError
	Hash                  []byte
	Start                 time.Time
	retries               atomic.Uint32
	err                   error
	announcedPeers        []p2p.PeerI
	status                metamorph_api.Status
	noStats               bool
	lastStatusUpdateNanos atomic.Int64
	Log                   []ProcessorResponseLog
}

func NewProcessorResponse(hash []byte) *ProcessorResponse {
	return &ProcessorResponse{
		Start:  time.Now(),
		Hash:   hash,
		status: metamorph_api.Status_UNKNOWN,
		Log: []ProcessorResponseLog{
			{
				DeltaT: 0,
				Status: metamorph_api.Status_UNKNOWN.String(),
			},
		},
	}
}

// NewProcessorResponseWithStatus creates a new ProcessorResponse with the given status.
// It is used when restoring the ProcessorResponseMap from the database.
func NewProcessorResponseWithStatus(hash []byte, status metamorph_api.Status) *ProcessorResponse {
	return &ProcessorResponse{
		Start:  time.Now(),
		Hash:   hash,
		status: status,
		Log: []ProcessorResponseLog{
			{
				DeltaT: 0,
				Status: status.String(),
			},
		},
	}
}

func NewProcessorResponseWithChannel(hash []byte, ch chan StatusAndError) *ProcessorResponse {
	return &ProcessorResponse{
		Start:  time.Now(),
		Hash:   hash,
		status: metamorph_api.Status_UNKNOWN,
		ch:     ch,
		Log: []ProcessorResponseLog{
			{
				DeltaT: 0,
				Status: metamorph_api.Status_UNKNOWN.String(),
			},
		},
	}
}

func (r *ProcessorResponse) SetPeers(peers []p2p.PeerI) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	r.announcedPeers = peers
}

func (r *ProcessorResponse) GetPeers() []p2p.PeerI {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.announcedPeers
}

func (r *ProcessorResponse) String() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.err != nil {
		return fmt.Sprintf("%x: %s [%s] %s", bt.ReverseBytes(r.Hash), r.Start.Format(time.RFC3339), r.status.String(), r.err.Error())
	}
	return fmt.Sprintf("%x: %s [%s]", bt.ReverseBytes(r.Hash), r.Start.Format(time.RFC3339), r.status.String())
}

func (r *ProcessorResponse) setStatus(status metamorph_api.Status, source string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.setStatusInternal(status, source)
}

func (r *ProcessorResponse) setStatusInternal(status metamorph_api.Status, source string) bool {
	r.status = status

	sae := StatusAndError{
		Hash:   r.Hash,
		Status: r.status,
	}

	ch := r.ch

	r.AddLog(status, source, "")

	if ch != nil {
		return utils.SafeSend(ch, sae)
	}

	return true
}

func (r *ProcessorResponse) GetStatus() metamorph_api.Status {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.status
}

func (r *ProcessorResponse) setErr(err error, source string) bool {
	r.mu.Lock()

	r.err = err

	sae := StatusAndError{
		Hash:   r.Hash,
		Status: r.status,
		Err:    err,
	}

	ch := r.ch

	r.AddLog(r.status, source, err.Error())

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
	r.status = status
	r.err = err

	sae := StatusAndError{
		Hash:   r.Hash,
		Status: r.status,
		Err:    err,
	}

	ch := r.ch

	r.AddLog(status, source, err.Error())

	if ch != nil {
		return utils.SafeSend(ch, sae)
	}

	return true
}

func (r *ProcessorResponse) GetErr() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.err
}

func (r *ProcessorResponse) Retries() uint32 {
	return r.retries.Load()
}

func (r *ProcessorResponse) IncrementRetry() uint32 {
	r.retries.Add(1)
	return r.retries.Load()
}

func (r *ProcessorResponse) AddLog(status metamorph_api.Status, source string, info string) {
	r.Log = append(r.Log, ProcessorResponseLog{
		DeltaT: time.Since(r.Start).Nanoseconds(),
		Status: status.String(),
		Source: source,
		Info:   info,
	})
}
