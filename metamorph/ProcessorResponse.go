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
	mu      deadlock.RWMutex
	ch      chan StatusAndError
	noStats bool

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
	return &ProcessorResponse{
		Start:  time.Now(),
		Hash:   hash,
		Status: metamorph_api.Status_UNKNOWN,
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
		Status: status,
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
		Status: metamorph_api.Status_UNKNOWN,
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

	ch := r.ch

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
