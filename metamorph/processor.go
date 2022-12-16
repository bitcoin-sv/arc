package metamorph

import (
	"context"
	"encoding/hex"
	"os"
	"sync/atomic"
	"time"

	pb "github.com/TAAL-GmbH/arc/metamorph/api"
	"github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/TAAL-GmbH/arc/p2p"
	"github.com/libsv/go-bt/v2"
	"github.com/ordishs/go-utils"

	"github.com/ordishs/go-utils/expiringmap"
	"github.com/ordishs/gocore"
)

type ProcessorRequest struct {
	*store.StoreData
	ResponseChannel chan *ProcessorResponse
}

func NewProcessorRequest(req *store.StoreData, responseChannel chan *ProcessorResponse) *ProcessorRequest {
	return &ProcessorRequest{
		req,
		responseChannel,
	}
}

type ProcessorResponse struct {
	ch     chan *ProcessorResponse
	Hash   []byte
	Start  time.Time
	Err    error
	Status pb.Status
}

type ProcessorStats struct {
	StartTime       time.Time
	UptimeMillis    int64
	WorkerCount     int32
	QueueLength     int32
	QueuedCount     int32
	ProcessedCount  int32
	ProcessedMillis int32
	ChannelMapSize  int32
}

type Processor struct {
	ch         chan *ProcessorRequest
	expiryChan chan *ProcessorResponse
	store      store.Store
	tx2ChMap   *expiringmap.ExpiringMap[string, *ProcessorResponse]
	pm         *p2p.PeerManager
	logger     *gocore.Logger

	startTime       time.Time
	workerCount     int32
	queueLength     atomic.Int32
	queuedCount     atomic.Int32
	processedCount  atomic.Int32
	processedMillis atomic.Int32
}

func NewProcessor(workerCount int32, s store.Store, pm *p2p.PeerManager) *Processor {
	logger := gocore.Log("processor")

	mapExpiryStr, _ := gocore.Config().Get("processorCacheExpiryTime", "10s")
	mapExpiry, err := time.ParseDuration(mapExpiryStr)
	if err != nil {
		logger.Fatalf("Invalid processorCacheExpiryTime: %s", mapExpiryStr)
	}

	logger.Infof("Starting processor with %d workers and cache expiry of %s", workerCount, mapExpiryStr)

	expiryChan := make(chan *ProcessorResponse)

	p := &Processor{
		startTime:   time.Now().UTC(),
		ch:          make(chan *ProcessorRequest),
		store:       s,
		tx2ChMap:    expiringmap.New[string](mapExpiry, expiryChan),
		workerCount: workerCount,
		expiryChan:  expiryChan,
		pm:          pm,
		logger:      logger,
	}

	go func() {
		for resp := range expiryChan {
			txidStr := hex.EncodeToString(bt.ReverseBytes(resp.Hash))
			logger.Infof("Resending expired tx: %s", txidStr)
			p.tx2ChMap.Set(txidStr, resp)
			p.pm.AnnounceNewTransaction(resp.Hash)
		}
	}()

	for i := int32(0); i < workerCount; i++ {
		go p.process(i)
	}

	return p
}

func (p *Processor) LoadUnseen() {
	err := p.store.GetUnseen(context.Background(), func(record *store.StoreData) {
		// add the records we have in the database, but that have not been processed, to the mempool watcher
		txidStr := hex.EncodeToString(bt.ReverseBytes(record.Hash))
		p.tx2ChMap.Set(txidStr, &ProcessorResponse{
			Hash:   record.Hash,
			Start:  time.Now(),
			Status: record.Status,
		})
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

func (p *Processor) SendStatusForTransaction(hashStr string, status pb.Status, err error) bool {
	resp, ok := p.tx2ChMap.Get(hashStr)
	if ok {
		// we have cached this transaction, so process accordingly
		resp.Status = status
		rejectReason := ""
		if err != nil {
			resp.Err = err
			rejectReason = err.Error()
		}

		err = p.store.UpdateStatus(context.Background(), resp.Hash, status, rejectReason)
		if err != nil {
			p.logger.Errorf("Error updating status for %s: %v", hashStr, err)
		}
		if resp.ch != nil {
			ok = utils.SafeSend(resp.ch, resp)
		}

		// Don't cache the channel if the transactionHandler is not listening any more
		// which will have been triggered by a status of SEEN or higher
		if status >= pb.Status_SENT_TO_NETWORK {
			p.processedCount.Add(1)
			p.processedMillis.Add(int32(time.Since(resp.Start).Milliseconds()))

			p.tx2ChMap.Delete(hashStr)
		}

		return ok
	} else if status > pb.Status_SENT_TO_NETWORK {
		if err != nil {
			// Print the error along with the status message
			p.logger.Infof("Received status %s for tx %s: %s", status.String(), hashStr, err.Error())
		} else {
			p.logger.Infof("Received status %s for tx %s", status.String(), hashStr)
		}
		// This is coming from zmq, after the transaction has been deleted from our tx2ChMap
		// It could be a "seen", "confirmed", "mined" or "rejected" status, but should finalize the tx
		var txIDBytes []byte
		txIDBytes, err = hex.DecodeString(hashStr)
		if err != nil {
			p.logger.Errorf("Error decoding txid %s: %v", hashStr, err)
			return false
		}

		hash := bt.ReverseBytes(txIDBytes)
		rejectReason := ""
		if err != nil {
			rejectReason = err.Error()
		}
		err = p.store.UpdateStatus(context.Background(), hash, status, rejectReason)
		if err != nil {
			p.logger.Errorf("Error updating status for %s: %v", hashStr, err)
		}
	}

	return false
}

func (p *Processor) GetStats() *ProcessorStats {
	return &ProcessorStats{
		StartTime:       p.startTime,
		UptimeMillis:    time.Since(p.startTime).Milliseconds(),
		WorkerCount:     p.workerCount,
		QueueLength:     p.queueLength.Load(),
		QueuedCount:     p.queuedCount.Load(),
		ProcessedCount:  p.processedCount.Load(),
		ProcessedMillis: p.processedMillis.Load(),
		ChannelMapSize:  int32(p.tx2ChMap.Len()),
	}
}

func (p *Processor) process(i int32) {
	for req := range p.ch {
		p.processTransaction(req)
	}
}

func (p *Processor) processTransaction(req *ProcessorRequest) {
	processorResponse := &ProcessorResponse{
		ch:     req.ResponseChannel,
		Hash:   req.Hash,
		Status: pb.Status_UNKNOWN,
		Start:  time.Now(),
	}

	p.queueLength.Add(-1)

	processorResponse.Status = pb.Status_RECEIVED
	utils.SafeSend(req.ResponseChannel, processorResponse)

	p.logger.Debugf("Adding channel for %x", bt.ReverseBytes(req.Hash))

	txidStr := hex.EncodeToString(bt.ReverseBytes(req.Hash))

	p.tx2ChMap.Set(txidStr, processorResponse)

	if err := p.store.Set(context.Background(), req.Hash, req.StoreData); err != nil {
		p.logger.Errorf("Error storing transaction %s: %v", txidStr, err)
		processorResponse.Err = err
		utils.SafeSend(req.ResponseChannel, processorResponse)
	} else {
		p.logger.Infof("Stored tx %s", txidStr)

		processorResponse.Status = pb.Status_STORED
		utils.SafeSend(req.ResponseChannel, processorResponse)

		p.pm.AnnounceNewTransaction(req.Hash)

		processorResponse.Status = pb.Status_ANNOUNCED_TO_NETWORK
		utils.SafeSend(req.ResponseChannel, processorResponse)
	}

	// update to the latest status of the transaction
	err := p.store.UpdateStatus(context.Background(), req.Hash, processorResponse.Status, "")
	if err != nil {
		p.logger.Errorf("Error updating status for %x: %v", bt.ReverseBytes(req.Hash), err)
	}
}

func (p *Processor) PrintStatsOnKeypress() func() {
	// The following util sets the terminal to non-canonical mode so that we can read
	// single characters from the terminal without having to press enter.
	ttyState := utils.DisableCanonicalMode(p.logger)

	// Print stats when the user presses a key...
	go func() {
		var b []byte = make([]byte, 1)
		for {
			_, _ = os.Stdin.Read(b)

			stats := p.GetStats()

			avg := 0.0
			if stats.ProcessedCount > 0 {
				avg = float64(stats.ProcessedMillis) / float64(stats.ProcessedCount)
			}

			p.logger.Infof(`Peer stats (started: %s):
------------------------
Workers:   %5d
Uptime:    %5.2f s
Queued:    %5d
Processed: %5d
Waiting:   %5d
Average:   %5.2f ms
MapSize:   %5d
------------------------
`,
				stats.StartTime.UTC().Format(time.RFC3339),
				stats.WorkerCount,
				float64(stats.UptimeMillis)/1000.0,
				stats.QueuedCount,
				stats.ProcessedCount,
				stats.QueueLength,
				avg,
				stats.ChannelMapSize,
			)
		}
	}()

	return func() {
		utils.RestoreTTY(ttyState)
	}
}
