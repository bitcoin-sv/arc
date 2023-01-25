package metamorph

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/libsv/go-bt/v2"
	"github.com/ordishs/go-utils"
)

type ProcessorResponse struct {
	externalCh      chan StatusAndError
	setStatusCh     chan metamorph_api.Status
	setErrCh        chan error
	requestStatusCh chan chan StatusAndError
	Hash            []byte
	Start           time.Time
	retries         atomic.Uint32
	err             error
	status          metamorph_api.Status
}

func NewProcessorResponse(hash []byte) *ProcessorResponse {
	r := &ProcessorResponse{
		Start:  time.Now(),
		Hash:   hash,
		status: metamorph_api.Status_UNKNOWN,
	}

	go func() {
		for {
			select {
			case status := <-r.setStatusCh:
				r.status = status

				if r.externalCh != nil {
					utils.SafeSend(r.externalCh, StatusAndError{
						Hash:   r.Hash,
						Status: r.status,
						Err:    r.err,
					})
				}

			case err := <-r.setErrCh:
				r.err = err

				if r.externalCh != nil {
					utils.SafeSend(r.externalCh, StatusAndError{
						Hash:   r.Hash,
						Status: r.status,
						Err:    r.err,
					})
				}

			case requestingCh := <-r.requestStatusCh:
				requestingCh <- StatusAndError{
					Hash:   r.Hash,
					Status: r.status,
					Err:    r.err,
				}

			}
		}
	}()

	return r
}

// NewProcessorResponseWithStatus creates a new ProcessorResponse with the given status.
// It is used when restoring the ProcessorResponseMap from the database.
func NewProcessorResponseWithStatus(hash []byte, status metamorph_api.Status) *ProcessorResponse {
	r := NewProcessorResponse(hash)
	r.status = status

	return r
}

func NewProcessorResponseWithChannel(hash []byte, ch chan StatusAndError) *ProcessorResponse {
	r := NewProcessorResponse(hash)
	r.externalCh = ch

	return r
}

func (r *ProcessorResponse) SetStatus(status metamorph_api.Status) bool {
	r.setStatusCh <- status

	return true
}

func (r *ProcessorResponse) SetErr(err error) bool {
	r.setErrCh <- err

	return true
}

func (r *ProcessorResponse) SetStatusAndError(status metamorph_api.Status, err error) bool {
	r.setStatusCh <- status
	r.setErrCh <- err

	return true
}

func (r *ProcessorResponse) GetStatus() metamorph_api.Status {
	ch := make(chan StatusAndError)
	r.requestStatusCh <- ch

	sae := <-ch
	return sae.Status
}

func (r *ProcessorResponse) GetStatusAndError() StatusAndError {
	ch := make(chan StatusAndError)
	r.requestStatusCh <- ch

	return <-ch
}

func (r *ProcessorResponse) GetErr() error {
	ch := make(chan StatusAndError)
	r.requestStatusCh <- ch

	sae := <-ch
	return sae.Err
}

func (r *ProcessorResponse) Retries() uint32 {
	return r.retries.Load()
}

func (r *ProcessorResponse) IncrementRetry() uint32 {
	r.retries.Add(1)
	return r.retries.Load()
}

func (r *ProcessorResponse) String() string {
	sae := r.GetStatusAndError()

	if r.err != nil {
		return fmt.Sprintf("%x: %s [%s] %s", bt.ReverseBytes(sae.Hash), r.Start.Format(time.RFC3339), sae.Status.String(), sae.Err.Error())
	}
	return fmt.Sprintf("%x: %s [%s]", bt.ReverseBytes(sae.Hash), r.Start.Format(time.RFC3339), sae.Status.String())
}
