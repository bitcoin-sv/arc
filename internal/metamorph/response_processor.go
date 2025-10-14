package metamorph

import (
	"context"
	"sync"

	"github.com/bsv-blockchain/go-bt/v2/chainhash"
)

type StatusResponse struct {
	ctx      context.Context
	statusCh chan StatusAndError

	Hash *chainhash.Hash
}

func NewStatusResponse(ctx context.Context, hash *chainhash.Hash, statusChannel chan StatusAndError) *StatusResponse {
	return &StatusResponse{
		ctx:      ctx,
		statusCh: statusChannel,
		Hash:     hash,
	}
}

func (r *StatusResponse) UpdateStatus(statusAndError StatusAndError) {
	if r.statusCh == nil || r.ctx == nil {
		return
	}

	select {
	case <-r.ctx.Done():
		return
	default:
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

func (p *ResponseProcessor) Add(statusResponse *StatusResponse) {
	if statusResponse.ctx == nil {
		return
	}

	_, loaded := p.resMap.LoadOrStore(*statusResponse.Hash, statusResponse)
	if loaded {
		return
	}

	// we no longer need status response object after response has been returned
	go func() {
		<-statusResponse.ctx.Done()
		p.resMap.Delete(*statusResponse.Hash)
	}()
}

func (p *ResponseProcessor) UpdateStatus(hash *chainhash.Hash, statusAndError StatusAndError) (found bool) {
	val, ok := p.resMap.Load(*hash)
	if !ok {
		return false
	}

	statusResponse, ok := val.(*StatusResponse)
	if !ok {
		return false
	}

	go statusResponse.UpdateStatus(statusAndError)
	return true
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
