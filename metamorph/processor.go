package metamorph

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/TAAL-GmbH/arc/p2p"
	"github.com/libsv/go-bt/v2"
	"github.com/ordishs/go-utils"

	"github.com/ordishs/gocore"
)

type ProcessorRequest struct {
	*store.StoreData
	ResponseChannel chan StatusAndError
}

func NewProcessorRequest(req *store.StoreData, responseChannel chan StatusAndError) *ProcessorRequest {
	return &ProcessorRequest{
		req,
		responseChannel,
	}
}

type StatusAndError struct {
	Hash   []byte
	Status metamorph_api.Status
	Err    error
}

type ProcessorResponse struct {
	mu     sync.RWMutex
	ch     chan StatusAndError
	Hash   []byte
	Start  time.Time
	err    error
	status metamorph_api.Status
}

func NewProcessorResponse(hash []byte) *ProcessorResponse {
	return &ProcessorResponse{
		Start:  time.Now(),
		Hash:   hash,
		status: metamorph_api.Status_UNKNOWN,
	}
}

// NewProcessorResponseWithStatus creates a new ProcessorResponse with the given status.
// It is used when restoring the ProcessorResponseMap from the database.
func NewProcessorResponseWithStatus(hash []byte, status metamorph_api.Status) *ProcessorResponse {
	return &ProcessorResponse{
		Start:  time.Now(),
		Hash:   hash,
		status: status,
	}
}

func NewProcessorResponseWithChannel(hash []byte, ch chan StatusAndError) *ProcessorResponse {
	return &ProcessorResponse{
		Start:  time.Now(),
		Hash:   hash,
		status: metamorph_api.Status_UNKNOWN,
		ch:     ch,
	}
}

func (r *ProcessorResponse) String() string {
	if r.err != nil {
		return fmt.Sprintf("%x: %s [%s] %s", utils.ReverseSlice(r.Hash), r.Start.Format(time.RFC3339), r.status.String(), r.err.Error())
	}
	return fmt.Sprintf("%x: %s [%s]", utils.ReverseSlice(r.Hash), r.Start.Format(time.RFC3339), r.status.String())
}

func (r *ProcessorResponse) SetStatus(status metamorph_api.Status) bool {
	r.mu.Lock()

	r.status = status

	sae := StatusAndError{
		Hash:   r.Hash,
		Status: r.status,
	}

	ch := r.ch

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

type ProcessorStats struct {
	StartTime       time.Time
	UptimeMillis    int64
	WorkerCount     int
	QueueLength     int32
	QueuedCount     int32
	ProcessedCount  int32
	ProcessedMillis int32
	ChannelMapSize  int32
}

type Processor struct {
	ch chan *ProcessorRequest
	//evictionChan chan []*ProcessorResponse
	store    store.Store
	tx2ChMap *ProcessorResponseMap
	pm       p2p.PeerManagerI
	logger   *gocore.Logger

	startTime       time.Time
	workerCount     int
	queueLength     atomic.Int32
	queuedCount     atomic.Int32
	processedCount  atomic.Int32
	processedMillis atomic.Int32
}

func NewProcessor(workerCount int, s store.Store, pm p2p.PeerManagerI) *Processor {
	if s == nil {
		panic("store cannot be nil")
	}
	if pm == nil {
		panic("peer manager cannot be nil")
	}

	logger := gocore.Log("processor")

	mapExpiryStr, _ := gocore.Config().Get("processorCacheExpiryTime", "24h")
	mapExpiry, err := time.ParseDuration(mapExpiryStr)
	if err != nil {
		logger.Fatalf("Invalid processorCacheExpiryTime: %s", mapExpiryStr)
	}

	logger.Infof("Starting processor with %d workers and cache expiry of %s", workerCount, mapExpiryStr)

	p := &Processor{
		startTime:   time.Now().UTC(),
		ch:          make(chan *ProcessorRequest),
		store:       s,
		tx2ChMap:    NewProcessorResponseMap(mapExpiry),
		workerCount: workerCount,
		pm:          pm,
		logger:      logger,
	}

	// go func() {
	// 	for items := range evictionChan {
	// 		for _, resp := range items {
	// 			if resp.GetStatus() < metamorph_api.Status_SEEN_ON_NETWORK {
	// 				txIDStr := hex.EncodeToString(bt.ReverseBytes(resp.Hash))
	// 				logger.Infof("Resending expired tx: %s", txIDStr)
	// 				p.tx2ChMap.Set(txIDStr, resp)
	// 				p.pm.AnnounceNewTransaction(resp.Hash)
	// 			}
	// 		}
	// 	}
	// }()

	for i := 0; i < workerCount; i++ {
		go p.process(i)
	}

	return p
}

func (p *Processor) LoadUnseen() {
	err := p.store.GetUnseen(context.Background(), func(record *store.StoreData) {
		// add the records we have in the database, but that have not been processed, to the mempool watcher
		txIDStr := hex.EncodeToString(bt.ReverseBytes(record.Hash))
		p.tx2ChMap.Set(txIDStr, NewProcessorResponseWithStatus(record.Hash, record.Status))

		p.queuedCount.Add(1)
		p.queueLength.Add(1)
		p.pm.AnnounceNewTransaction(record.Hash)
	})
	if err != nil {
		p.logger.Errorf("Error iterating through stored transactions: %v", err)
	}
}

func (p *Processor) ProcessTransaction(req *ProcessorRequest) {
	p.queuedCount.Add(1)
	p.queueLength.Add(1)

	p.ch <- req
}

func (p *Processor) SendStatusMinedForTransaction(hash []byte, blockHash []byte, blockHeight int32) (bool, error) {
	hashStr := hex.EncodeToString(bt.ReverseBytes(hash))

	err := p.store.UpdateMined(context.Background(), hash, blockHash, blockHeight)
	if err != nil {
		if err != store.ErrNotFound {
			p.logger.Errorf("Error updating status for %s: %v", hashStr, err)
			return false, err
		}
	}

	// remove the transaction from the tx map, regardless of status
	resp, ok := p.tx2ChMap.Get(hashStr)
	if ok {
		p.processedCount.Add(1)
		p.processedMillis.Add(int32(time.Since(resp.Start).Milliseconds()))
		p.tx2ChMap.Delete(hashStr)
	}

	return true, nil
}

func (p *Processor) SendStatusForTransaction(hashStr string, status metamorph_api.Status, statusErr error) (bool, error) {
	resp, ok := p.tx2ChMap.Get(hashStr)
	if ok {
		// we have cached this transaction, so process accordingly
		rejectReason := ""
		if statusErr != nil {
			rejectReason = statusErr.Error()
		}

		err := p.store.UpdateStatus(context.Background(), resp.Hash, status, rejectReason)
		if err != nil {
			p.logger.Errorf("Error updating status for %s: %v", hashStr, err)
		}

		if statusErr != nil {
			ok = resp.SetStatusAndError(status, statusErr)
		} else {
			ok = resp.SetStatus(status)
		}

		// Don't cache the channel if the transactionHandler is not listening anymore
		// which will have been triggered by a status of SEEN or higher
		if status >= metamorph_api.Status_SEEN_ON_NETWORK {
			p.processedCount.Add(1)
			p.processedMillis.Add(int32(time.Since(resp.Start).Milliseconds()))
			p.tx2ChMap.Delete(hashStr)
		}

		return ok, nil
	} else if status > metamorph_api.Status_SEEN_ON_NETWORK {
		if statusErr != nil {
			// Print the error along with the status message
			p.logger.Infof("Received status %s for tx %s: %s", status.String(), hashStr, statusErr.Error())
		} else {
			p.logger.Infof("Received status %s for tx %s", status.String(), hashStr)
		}
		// This is coming from zmq, after the transaction has been deleted from our tx2ChMap
		// It could be a "seen", "confirmed", "mined" or "rejected" status, but should finalize the tx
		hash, err := utils.DecodeAndReverseHexString(hashStr)
		if err != nil {
			p.logger.Errorf("Error decoding txID %s: %v", hashStr, err)
			return false, err
		}

		rejectReason := ""
		if statusErr != nil {
			rejectReason = statusErr.Error()
		}
		err = p.store.UpdateStatus(context.Background(), hash, status, rejectReason)
		if err != nil {
			if err != store.ErrNotFound {
				p.logger.Errorf("Error updating status for %s: %v", hashStr, err)
				return false, err
			}
		}

		return true, nil
	}

	return false, nil
}

func (p *Processor) process(_ int) {
	for req := range p.ch {
		p.processTransaction(req)
	}
}

func (p *Processor) processTransaction(req *ProcessorRequest) {
	p.queueLength.Add(-1)

	p.logger.Debugf("Adding channel for %x", bt.ReverseBytes(req.Hash))

	processorResponse := NewProcessorResponseWithChannel(req.Hash, req.ResponseChannel)
	processorResponse.SetStatus(metamorph_api.Status_RECEIVED)

	txIDStr := hex.EncodeToString(bt.ReverseBytes(req.Hash))
	p.tx2ChMap.Set(txIDStr, processorResponse)

	if err := p.store.Set(context.Background(), req.Hash, req.StoreData); err != nil {
		p.logger.Errorf("Error storing transaction %s: %v", txIDStr, err)
		processorResponse.SetErr(err)
	} else {
		p.logger.Infof("Stored tx %s", txIDStr)

		processorResponse.SetStatus(metamorph_api.Status_STORED)

		p.pm.AnnounceNewTransaction(req.Hash)

		processorResponse.SetStatus(metamorph_api.Status_ANNOUNCED_TO_NETWORK)
	}

	// update to the latest status of the transaction
	err := p.store.UpdateStatus(context.Background(), req.Hash, processorResponse.GetStatus(), "")
	if err != nil {
		p.logger.Errorf("Error updating status for %x: %v", bt.ReverseBytes(req.Hash), err)
	}
}
