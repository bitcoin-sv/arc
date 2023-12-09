package metamorph

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bitcoin-sv/arc/blocktx"
	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/callbacker/callbacker_api"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/metamorph/processor_response"
	"github.com/bitcoin-sv/arc/metamorph/store"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	tracingLog "github.com/opentracing/opentracing-go/log"
	"github.com/ordishs/go-utils/stat"
	"github.com/ordishs/gocore"
)

const (
	// number of times we will retry announcing transaction if we haven't seen it on the network
	MaxRetries = 15
	// length of interval for checking transactions if they are seen on the network
	// if not we resend them again for a few times
	unseenTransactionRebroadcastingInterval = 60
	processExpiredSeenTxsIntervalDefault    = 5 * time.Minute
	mapExpiryTimeDefault                    = 24 * time.Hour
	LogLevelDefault                         = slog.LevelInfo

	failedToUpdateStatus       = "Failed to update status"
	dataRetentionPeriodDefault = 14 * 24 * time.Hour // 14 days
)

var (
	statusValueMap = map[metamorph_api.Status]int{
		metamorph_api.Status_UNKNOWN:                0,
		metamorph_api.Status_QUEUED:                 1,
		metamorph_api.Status_RECEIVED:               2,
		metamorph_api.Status_STORED:                 3,
		metamorph_api.Status_ANNOUNCED_TO_NETWORK:   4,
		metamorph_api.Status_REQUESTED_BY_NETWORK:   5,
		metamorph_api.Status_SENT_TO_NETWORK:        6,
		metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL: 7,
		metamorph_api.Status_ACCEPTED_BY_NETWORK:    8,
		metamorph_api.Status_SEEN_ON_NETWORK:        9,
		metamorph_api.Status_REJECTED:               10,
		metamorph_api.Status_MINED:                  11,
		metamorph_api.Status_CONFIRMED:              12,
	}
)

type Processor struct {
	store               store.MetamorphStore
	cbChannel           chan *callbacker_api.Callback
	pm                  p2p.PeerManagerI
	btc                 blocktx.ClientI
	logger              *slog.Logger
	logFile             string
	mapExpiryTime       time.Duration
	dataRetentionPeriod time.Duration
	now                 func() time.Time

	startTime          time.Time
	queueLength        atomic.Int32
	queuedCount        atomic.Int32
	stored             *stat.AtomicStat
	announcedToNetwork *stat.AtomicStats
	requestedByNetwork *stat.AtomicStats
	sentToNetwork      *stat.AtomicStats
	acceptedByNetwork  *stat.AtomicStats
	seenOnNetwork      *stat.AtomicStats
	rejected           *stat.AtomicStats
	mined              *stat.AtomicStat
	retries            *stat.AtomicStat
}

type Option func(f *Processor)

func NewProcessor(s store.MetamorphStore, pm p2p.PeerManagerI,
	cbChannel chan *callbacker_api.Callback, btc blocktx.ClientI, opts ...Option) (*Processor, error) {
	if s == nil {
		return nil, errors.New("store cannot be nil")
	}

	if pm == nil {
		return nil, errors.New("peer manager cannot be nil")
	}

	p := &Processor{
		startTime:           time.Now().UTC(),
		store:               s,
		cbChannel:           cbChannel,
		pm:                  pm,
		btc:                 btc,
		dataRetentionPeriod: dataRetentionPeriodDefault,
		mapExpiryTime:       mapExpiryTimeDefault,
		now:                 time.Now,

		stored:             stat.NewAtomicStat(),
		announcedToNetwork: stat.NewAtomicStats(),
		requestedByNetwork: stat.NewAtomicStats(),
		sentToNetwork:      stat.NewAtomicStats(),
		acceptedByNetwork:  stat.NewAtomicStats(),
		seenOnNetwork:      stat.NewAtomicStats(),
		rejected:           stat.NewAtomicStats(),
		mined:              stat.NewAtomicStat(),
		retries:            stat.NewAtomicStat(),
	}

	p.logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: LogLevelDefault})).With(slog.String("service", "mtm"))

	// apply options to processor
	for _, opt := range opts {
		opt(p)
	}

	// expired transactions will be deleted after 14 days anyway, so why bother?

	//_ = newPrometheusCollector(p)

	return p, nil
}

func (p *Processor) Set(ctx context.Context, req *ProcessorRequest) error {
	// we need to decouple the Context from the request, so that we don't get cancelled
	// when the request is cancelled
	callerSpan := opentracing.SpanFromContext(ctx)
	newctx := opentracing.ContextWithSpan(context.Background(), callerSpan)
	_, spanCtx := opentracing.StartSpanFromContext(newctx, "Processor:processTransaction")
	return p.store.Set(spanCtx, req.Data.Hash[:], req.Data)
}

// Shutdown closes all channels and goroutines gracefully
func (p *Processor) Shutdown() {
	p.logger.Info("Shutting down processor")

	if p.cbChannel != nil {
		close(p.cbChannel)
	}
}

// GetPeers returns a list of connected and a list of disconnected peers
func (p *Processor) GetPeers() ([]string, []string) {
	peers := p.pm.GetPeers()
	peersConnected := make([]string, 0, len(peers))
	peersDisconnected := make([]string, 0, len(peers))
	for _, peer := range peers {
		if peer.Connected() {
			peersConnected = append(peersConnected, peer.String())
		} else {
			peersDisconnected = append(peersDisconnected, peer.String())
		}
	}

	return peersConnected, peersDisconnected
}

func (p *Processor) LoadUnmined() {
	span, spanCtx := opentracing.StartSpanFromContext(context.Background(), "Processor:LoadUnmined")
	defer span.Finish()

	err := p.store.GetUnmined(spanCtx, func(record *store.StoreData) {

		if !p.store.IsCentralised() {
			return
		}

		// add the records we have in the database, but that have not been processed, to the mempool watcher
		pr := processor_response.NewProcessorResponseWithStatus(record.Hash, record.Status)
		pr.NoStats = true
		pr.Start = record.StoredAt

		switch record.Status {
		case metamorph_api.Status_STORED:
			// announce the transaction to the network
			pr.SetPeers(p.pm.AnnounceTransaction(record.Hash, nil))

			err := p.store.UpdateStatus(spanCtx, record.Hash, metamorph_api.Status_ANNOUNCED_TO_NETWORK, "")
			if err != nil {
				p.logger.Error(failedToUpdateStatus, slog.String("hash", record.Hash.String()), slog.String("err", err.Error()))
			}
		case metamorph_api.Status_ANNOUNCED_TO_NETWORK:
			// we only announced the transaction, but we did not receive a SENT_TO_NETWORK response
			// let's send a GETDATA message to the network to check whether the transaction is actually there
			p.pm.RequestTransaction(record.Hash)
		case metamorph_api.Status_SENT_TO_NETWORK:
			p.pm.RequestTransaction(record.Hash)
		case metamorph_api.Status_SEEN_ON_NETWORK:
			// could it already be mined, and we need to get it from BlockTx?
			transactionResponse, err := p.btc.GetTransactionBlock(context.Background(), &blocktx_api.Transaction{Hash: record.Hash[:]})

			if err != nil {
				p.logger.Error("failed to get transaction block", slog.String("hash", record.Hash.String()), slog.String("err", err.Error()))
				return
			}

			if transactionResponse == nil || transactionResponse.BlockHeight <= 0 {
				return
			}

			// we have a mined transaction, let's update the status
			blockHash, err := chainhash.NewHash(transactionResponse.BlockHash)
			if err != nil {
				p.logger.Error("Failed to convert block hash", slog.String("err", err.Error()))
				return
			}

			if err = p.SendStatusMinedForTransaction(record.Hash, blockHash, transactionResponse.BlockHeight); err != nil {
				p.logger.Error("Failed to update status for mined transaction", slog.String("err", err.Error()))
			}
		}
	})
	if err != nil {
		p.logger.Error("Failed to iterate through stored transactions", slog.String("err", err.Error()))
	}
}

func (p *Processor) SendStatusMinedForTransaction(hash *chainhash.Hash, blockHash *chainhash.Hash, blockHeight uint64) error {
	span, spanCtx := opentracing.StartSpanFromContext(context.Background(), "Processor:SendStatusMinedForTransaction")
	defer span.Finish()

	if err := p.store.UpdateMined(spanCtx, hash, blockHash, blockHeight); err != nil {
		return err
	}
	data, err := p.store.Get(spanCtx, hash[:])
	if err != nil {
		return err
	}

	if data != nil && data.CallbackUrl != "" {
		p.cbChannel <- &callbacker_api.Callback{
			Hash:        data.Hash[:],
			Url:         data.CallbackUrl,
			Token:       data.CallbackToken,
			Status:      int32(data.Status),
			BlockHash:   data.BlockHash[:],
			BlockHeight: data.BlockHeight,
		}
	}

	return nil
}

func (p *Processor) SendStatusForTransaction(hash *chainhash.Hash, status metamorph_api.Status, source string, statusErr error) error {
	rejectReason := ""
	if statusErr != nil {
		rejectReason = statusErr.Error()
	}
	err := p.store.UpdateStatus(context.TODO(), hash, status, rejectReason)
	if err != nil {
		return err
	}
	return nil
}

func (p *Processor) ProcessTransaction(ctx context.Context, req *ProcessorRequest) {
	startNanos := time.Now().UnixNano()

	// we need to decouple the Context from the request, so that we don't get cancelled
	// when the request is cancelled
	callerSpan := opentracing.SpanFromContext(ctx)
	newctx := opentracing.ContextWithSpan(context.Background(), callerSpan)
	span, spanCtx := opentracing.StartSpanFromContext(newctx, "Processor:processTransaction")
	defer span.Finish()

	p.queuedCount.Add(1)

	p.logger.Debug("Adding channel", slog.String("hash", req.Data.Hash.String()))

	processorResponse := processor_response.NewProcessorResponseWithChannel(req.Data.Hash, req.ResponseChannel)

	// STEP 1: RECEIVED
	processorResponse.UpdateStatus(&processor_response.ProcessorResponseStatusUpdate{
		Status: metamorph_api.Status_RECEIVED,
		Source: "processor",
		Callback: func(err error) {
			if err != nil {
				p.logger.Error("Error received when setting status", slog.String("status", metamorph_api.Status_RECEIVED.String()), slog.String("hash", req.Data.Hash.String()), slog.String("err", err.Error()))
				return
			}

			// STEP 2: STORED
			processorResponse.UpdateStatus(&processor_response.ProcessorResponseStatusUpdate{
				Status: metamorph_api.Status_STORED,
				Source: "processor",
				UpdateStore: func() error {
					return p.store.Set(spanCtx, req.Data.Hash[:], req.Data)
				},
				Callback: func(err error) {
					if err != nil {
						span.SetTag(string(ext.Error), true)
						p.logger.Error("Failed to store transaction", slog.String("hash", req.Data.Hash.String()), slog.String("err", err.Error()))
						span.LogFields(tracingLog.Error(err))
						return
					}

					// STEP 3: ANNOUNCED_TO_NETWORK
					peers := p.pm.AnnounceTransaction(req.Data.Hash, nil)
					processorResponse.SetPeers(peers)

					peersStr := make([]string, 0, len(peers))
					if len(peers) > 0 {
						for _, peer := range peers {
							peersStr = append(peersStr, peer.String())
						}
					}

					processorResponse.UpdateStatus(&processor_response.ProcessorResponseStatusUpdate{
						Status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
						Source: strings.Join(peersStr, ", "),
						UpdateStore: func() error {
							return p.store.UpdateStatus(spanCtx, req.Data.Hash, metamorph_api.Status_ANNOUNCED_TO_NETWORK, "")
						},
						Callback: func(err error) {
							duration := time.Since(processorResponse.Start)
							for _, peerStr := range peersStr {
								p.announcedToNetwork.AddDuration(peerStr, duration)
							}

							gocore.NewStat("processor").AddTime(startNanos)
						},
					})
				},
			})
		},
	})
}
