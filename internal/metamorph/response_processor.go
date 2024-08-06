package metamorph

import (
	"sync"
	"time"

	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

type StatusResponse struct {
	statusCh chan StatusAndError
	mu       sync.RWMutex

	Status metamorph_api.Status
	Hash   *chainhash.Hash
	Err    error
}

func NewStatusResponse(hash *chainhash.Hash, statusChannel chan StatusAndError) *StatusResponse {
	return &StatusResponse{
		statusCh: statusChannel,
		Hash:     hash,
		Status:   metamorph_api.Status_RECEIVED, // if it got to the point of creating this object, the status is RECEIVED
	}
}

func (r *StatusResponse) UpdateStatus(status metamorph_api.Status, err error) {
	r.mu.Lock()

	r.Status = status
	r.Err = err

	r.mu.Unlock()

	if r.statusCh != nil {
		r.statusCh <- StatusAndError{
			Hash:   r.Hash,
			Status: status,
			Err:    err,
		}
	}
}

type ResponseProcessor struct {
	responseMap map[*chainhash.Hash]*StatusResponse
}

func NewResponseProcessor() *ResponseProcessor {
	return &ResponseProcessor{
		responseMap: make(map[*chainhash.Hash]*StatusResponse),
	}
}

func (p *ResponseProcessor) Add(statusRespone *StatusResponse, timeout time.Duration) {
	p.responseMap[statusRespone.Hash] = statusRespone

	// we no longer need status response object after response has been returned
	go func() {
		time.Sleep(timeout)
		delete(p.responseMap, statusRespone.Hash)
	}()
}

func (p *ResponseProcessor) UpdateStatus(hash *chainhash.Hash, status metamorph_api.Status, err error) {
	res, ok := p.responseMap[hash]
	if !ok {
		return
	}

	res.UpdateStatus(status, err)
}
