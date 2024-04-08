package processor_response

import (
	"fmt"
	"time"

	"github.com/bitcoin-sv/arc/pkg/metamorph/metamorph_api"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/ordishs/go-utils"
	"github.com/sasha-s/go-deadlock"
)

type StatusAndError struct {
	Hash   *chainhash.Hash
	Status metamorph_api.Status
	Err    error
}

type ProcessorResponseStatusUpdate struct {
	Status    metamorph_api.Status
	StatusErr error
}

type ProcessorResponse struct {
	callerCh chan StatusAndError
	Hash     *chainhash.Hash `json:"hash"`
	// The following fields are protected by the mutex
	mu     deadlock.RWMutex
	Err    error                `json:"err"`
	Status metamorph_api.Status `json:"status"`
}

func NewProcessorResponse(hash *chainhash.Hash) *ProcessorResponse {
	return newProcessorResponse(hash, metamorph_api.Status_RECEIVED, nil, time.Second)
}

func NewProcessorResponseWithChannel(hash *chainhash.Hash, ch chan StatusAndError, timeout time.Duration) *ProcessorResponse {
	return newProcessorResponse(hash, metamorph_api.Status_RECEIVED, ch, timeout)
}

func newProcessorResponse(hash *chainhash.Hash, status metamorph_api.Status, ch chan StatusAndError, timeout time.Duration) *ProcessorResponse {
	pr := &ProcessorResponse{
		Hash:     hash,
		Status:   status,
		callerCh: ch,
	}

	return pr
}

func (r *ProcessorResponse) UpdateStatus(statusUpdate *ProcessorResponseStatusUpdate) {
	if statusUpdate.StatusErr != nil {
		_ = r.setStatusAndError(statusUpdate.Status, statusUpdate.StatusErr)
	} else {
		_ = r.setStatus(statusUpdate.Status)
	}

}

func (r *ProcessorResponse) String() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.Err != nil {
		return fmt.Sprintf("%v: [%s] %s", r.Hash, r.Status.String(), r.Err.Error())
	}
	return fmt.Sprintf("%v: [%s]", r.Hash, r.Status.String())
}

func (r *ProcessorResponse) setStatus(status metamorph_api.Status) bool {
	r.mu.Lock()
	r.Status = status

	sae := StatusAndError{
		Hash:   r.Hash,
		Status: r.Status,
	}
	r.mu.Unlock()

	if r.callerCh != nil {
		return utils.SafeSend(r.callerCh, sae)
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

	r.mu.Unlock()

	if r.callerCh != nil {
		return utils.SafeSend(r.callerCh, sae)
	}

	return true
}

func (r *ProcessorResponse) GetErr() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.Err
}

func (r *ProcessorResponse) setStatusAndError(status metamorph_api.Status, err error) bool {
	r.Status = status
	r.Err = err

	sae := StatusAndError{
		Hash:   r.Hash,
		Status: r.Status,
		Err:    err,
	}

	if r.callerCh != nil {
		return utils.SafeSend(r.callerCh, sae)
	}

	return true
}
