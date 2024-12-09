package metamorph

import (
	"context"
	"sync"

	"github.com/bitcoin-sv/arc/internal/cache"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

const CacheRegisteredTxsHash = "mtm-registered-txs"

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
	resMap     sync.Map
	cacheStore cache.Store
}

func NewResponseProcessor(c cache.Store) *ResponseProcessor {
	return &ResponseProcessor{cacheStore: c}
}

func (p *ResponseProcessor) Add(statusResponse *StatusResponse) {
	if statusResponse.ctx == nil {
		return
	}

	_, loaded := p.resMap.LoadOrStore(*statusResponse.Hash, statusResponse)
	if loaded {
		return
	}

	p.saveToCache(statusResponse.Hash)

	// we no longer need status response object after response has been returned
	go func() {
		<-statusResponse.ctx.Done()
		p.resMap.Delete(*statusResponse.Hash)
	}()
}

func (p *ResponseProcessor) UpdateStatus(hash *chainhash.Hash, statusAndError StatusAndError) (found bool) {
	val, ok := p.resMap.Load(*hash)
	if !ok {
		// if we don't have the transaction in memory, check cache
		return p.foundInCache(hash)
	}

	statusResponse, ok := val.(*StatusResponse)
	if !ok {
		return false
	}

	go statusResponse.UpdateStatus(statusAndError)

	// if tx is rejected, we don't expect any more status updates - remove from cache
	if statusAndError.Status == metamorph_api.Status_REJECTED {
		p.delFromCache(hash)
	}

	return true
}

func (p *ResponseProcessor) saveToCache(hash *chainhash.Hash) {
	if p.cacheStore.IsShared() {
		_ = p.cacheStore.MapSet(CacheRegisteredTxsHash, hash.String(), []byte("1"))
	}
}

func (p *ResponseProcessor) foundInCache(hash *chainhash.Hash) (found bool) {
	if p.cacheStore.IsShared() {
		value, _ := p.cacheStore.MapGet(CacheRegisteredTxsHash, hash.String())
		return value != nil
	}

	// if we don't have a shared cache running, treat all transactions as found
	return true
}

func (p *ResponseProcessor) delFromCache(hash *chainhash.Hash) {
	if p.cacheStore.IsShared() {
		_ = p.cacheStore.MapDel(CacheRegisteredTxsHash, hash.String())
	}
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
