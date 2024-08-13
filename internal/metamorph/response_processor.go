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

	Status       metamorph_api.Status
	Hash         *chainhash.Hash
	Err          error
	CompetingTxs []string
}

func NewStatusResponse(hash *chainhash.Hash, statusChannel chan StatusAndError) *StatusResponse {
	return &StatusResponse{
		statusCh: statusChannel,
		Hash:     hash,
		Status:   metamorph_api.Status_RECEIVED, // if it got to the point of creating this object, the status is RECEIVED
	}
}

func (r *StatusResponse) UpdateStatus(statusAndError StatusAndError) {
	r.mu.Lock()

	r.Status = statusAndError.Status
	r.Err = statusAndError.Err
	r.CompetingTxs = statusAndError.CompetingTxs

	r.mu.Unlock()

	if r.statusCh != nil {
		r.statusCh <- StatusAndError{
			Hash:         r.Hash,
			Status:       statusAndError.Status,
			Err:          statusAndError.Err,
			CompetingTxs: statusAndError.CompetingTxs,
		}
	}
}

type ResponseProcessor struct {
	resMap sync.Map
}

func NewResponseProcessor() *ResponseProcessor {
	return &ResponseProcessor{}
}

func (p *ResponseProcessor) Add(statusResponse *StatusResponse, timeout time.Duration) {
	_, loaded := p.resMap.LoadOrStore(*statusResponse.Hash, statusResponse)
	if loaded {
		return
	}

	// we no longer need status response object after response has been returned
	go func() {
		time.Sleep(timeout)
		p.resMap.Delete(*statusResponse.Hash)
	}()
}

func (p *ResponseProcessor) UpdateStatus(hash *chainhash.Hash, statusAndError StatusAndError) {
	val, ok := p.resMap.Load(*hash)
	if !ok {
		return
	}

	statusResponse, ok := val.(*StatusResponse)
	if !ok {
		return
	}

	statusResponse.UpdateStatus(statusAndError)
}

// use for tests only
func (p *ResponseProcessor) getMap() map[chainhash.Hash]*StatusResponse {
	retMap := make(map[chainhash.Hash]*StatusResponse)

	p.resMap.Range(func(key, val any) bool {
		k, ok := key.(chainhash.Hash)
		if !ok {
			return true // continue
		}

		v, ok := val.(*StatusResponse)
		if !ok {
			return true // continue
		}

		retMap[k] = v

		return true // continue
	})

	return retMap
}

func (p *ResponseProcessor) getMapLen() int {
	var length int

	p.resMap.Range(func(_, _ any) bool {
		length++
		return true
	})

	return length
}
