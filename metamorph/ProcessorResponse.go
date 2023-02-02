package metamorph

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/p2p"
	"github.com/libsv/go-bt/v2"
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
	statusStats           map[int32]int64
	noStats               bool
	lastStatusUpdateNanos atomic.Int64
}

func NewProcessorResponse(hash []byte) *ProcessorResponse {
	return &ProcessorResponse{
		Start:  time.Now(),
		Hash:   hash,
		status: metamorph_api.Status_UNKNOWN,
		statusStats: map[int32]int64{
			int32(metamorph_api.Status_UNKNOWN): time.Now().UnixNano(),
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
		statusStats: map[int32]int64{
			int32(status): time.Now().UnixNano(),
		},
	}
}

func NewProcessorResponseWithChannel(hash []byte, ch chan StatusAndError) *ProcessorResponse {
	return &ProcessorResponse{
		Start:  time.Now(),
		Hash:   hash,
		status: metamorph_api.Status_UNKNOWN,
		statusStats: map[int32]int64{
			int32(metamorph_api.Status_UNKNOWN): time.Now().UnixNano(),
		},
		ch: ch,
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

func (r *ProcessorResponse) GetStats() map[int32]int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.statusStats
}

func (r *ProcessorResponse) String() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.err != nil {
		return fmt.Sprintf("%x: %s [%s] %s", bt.ReverseBytes(r.Hash), r.Start.Format(time.RFC3339), r.status.String(), r.err.Error())
	}
	return fmt.Sprintf("%x: %s [%s]", bt.ReverseBytes(r.Hash), r.Start.Format(time.RFC3339), r.status.String())
}

func (r *ProcessorResponse) SetStatus(status metamorph_api.Status) bool {
	r.mu.Lock()

	r.status = status

	sae := StatusAndError{
		Hash:   r.Hash,
		Status: r.status,
	}

	ch := r.ch

	if _, ok := r.statusStats[int32(status)]; !ok {
		r.statusStats[int32(status)] = time.Now().UnixNano()
	}

	r.mu.Unlock()

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

func (r *ProcessorResponse) SetErr(err error) bool {
	r.mu.Lock()

	r.err = err

	sae := StatusAndError{
		Hash:   r.Hash,
		Status: r.status,
		Err:    err,
	}

	ch := r.ch

	r.mu.Unlock()

	if ch != nil {
		return utils.SafeSend(ch, sae)
	}

	return true
}

func (r *ProcessorResponse) SetStatusAndError(status metamorph_api.Status, err error) bool {
	r.mu.Lock()

	r.status = status
	r.err = err

	sae := StatusAndError{
		Hash:   r.Hash,
		Status: r.status,
		Err:    err,
	}

	ch := r.ch

	if _, ok := r.statusStats[int32(status)]; !ok {
		r.statusStats[int32(status)] = time.Now().UnixNano()
	}

	r.mu.Unlock()

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
