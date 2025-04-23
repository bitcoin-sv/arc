package blocktx

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"sync"
	"sync/atomic"
	"time"

	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/libsv/go-bc"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"

	"github.com/bitcoin-sv/arc/internal/blocktx/bcnet"
	"github.com/bitcoin-sv/arc/internal/blocktx/bcnet/blocktx_p2p"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/bitcoin-sv/arc/internal/mq"
	"github.com/bitcoin-sv/arc/pkg/tracing"
)

var (
	ErrFailedToSubscribeToTopic            = errors.New("failed to subscribe to register topic")
	ErrFailedToGetBlockTransactions        = errors.New("failed to get block transactions")
	ErrFailedToParseBlockHash              = errors.New("failed to parse block hash")
	ErrFailedToInsertBlockTransactions     = errors.New("failed to insert block transactions")
	ErrBlockAlreadyExists                  = errors.New("block already exists in the database")
	ErrUnexpectedBlockStatus               = errors.New("unexpected block status")
	ErrFailedToProcessBlock                = errors.New("failed to process block")
	ErrFailedToCalculateMissingMerklePaths = errors.New("failed to calculate missing merkle paths")
	ErrFailedToUnmarshalMessage            = errors.New("failed to unmarshal message")
)

const (
	transactionStoringBatchsizeDefault = 10000
	maxRequestBlocks                   = 10
	maxBlocksInProgress                = 1
	registerTxsIntervalDefault         = time.Second * 10
	registerRequestTxsIntervalDefault  = time.Second * 5
	registerTxsBatchSizeDefault        = 100
	waitForBlockProcessing             = 5 * time.Minute
	parallellism                       = 5
	publishMinedMessageSizeDefault     = 256
)

type Processor struct {
	hostname                    string
	blockRequestCh              chan blocktx_p2p.BlockRequest
	blockProcessCh              chan *bcnet.BlockMessage
	store                       store.BlocktxStore
	logger                      *slog.Logger
	transactionStorageBatchSize int
	dataRetentionDays           int
	mqClient                    mq.MessageQueueClient
	registerTxsChan             chan []byte
	registerTxsInterval         time.Duration
	registerRequestTxsInterval  time.Duration
	registerTxsBatchSize        int
	tracingEnabled              bool
	tracingAttributes           []attribute.KeyValue

	incomingIsLongest       bool
	publishMinedMessageSize int

	now                        func() time.Time
	maxBlockProcessingDuration time.Duration

	waitGroup *sync.WaitGroup
	cancelAll context.CancelFunc
	ctx       context.Context
}

func NewProcessor(
	logger *slog.Logger,
	storeI store.BlocktxStore,
	blockRequestCh chan blocktx_p2p.BlockRequest,
	blockProcessCh chan *bcnet.BlockMessage,
	opts ...func(*Processor),
) (*Processor, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	p := &Processor{
		store:                       storeI,
		logger:                      logger.With(slog.String("module", "processor")),
		blockRequestCh:              blockRequestCh,
		blockProcessCh:              blockProcessCh,
		transactionStorageBatchSize: transactionStoringBatchsizeDefault,
		registerTxsInterval:         registerTxsIntervalDefault,
		registerRequestTxsInterval:  registerRequestTxsIntervalDefault,
		registerTxsBatchSize:        registerTxsBatchSizeDefault,
		maxBlockProcessingDuration:  waitForBlockProcessing,
		hostname:                    hostname,
		publishMinedMessageSize:     publishMinedMessageSizeDefault,
		now:                         time.Now,
		waitGroup:                   &sync.WaitGroup{},
	}

	for _, opt := range opts {
		opt(p)
	}

	ctx, cancelAll := context.WithCancel(context.Background())
	p.cancelAll = cancelAll
	p.ctx = ctx

	return p, nil
}

func (p *Processor) Start() error {
	err := p.mqClient.QueueSubscribe(mq.RegisterTxTopic, func(msg []byte) error {
		p.registerTxsChan <- msg
		return nil
	})
	if err != nil {
		return errors.Join(ErrFailedToSubscribeToTopic, fmt.Errorf("topic: %s", mq.RegisterTxTopic), err)
	}

	err = p.mqClient.QueueSubscribe(mq.RegisterTxsTopic, func(msg []byte) error {
		serialized := &blocktx_api.Transactions{}
		err := proto.Unmarshal(msg, serialized)
		if err != nil {
			return errors.Join(ErrFailedToUnmarshalMessage, fmt.Errorf("topic: %s", mq.RegisterTxsTopic), err)
		}

		for _, tx := range serialized.Transactions {
			p.registerTxsChan <- tx.Hash
		}

		return nil
	})
	if err != nil {
		return errors.Join(ErrFailedToSubscribeToTopic, fmt.Errorf("topic: %s", mq.RegisterTxsTopic), err)
	}

	p.StartBlockRequesting()
	p.StartBlockProcessing()
	p.StartProcessRegisterTxs()

	return nil
}

func (p *Processor) StartBlockRequesting() {
	p.waitGroup.Add(1)

	go func() {
		defer p.waitGroup.Done()
		for {
			select {
			case <-p.ctx.Done():
				return
			case req := <-p.blockRequestCh:
				hash := req.Hash
				peer := req.Peer

				// lock block for the current instance to process
				processedBy, err := p.store.SetBlockProcessing(p.ctx, hash, p.hostname, p.maxBlockProcessingDuration, maxBlocksInProgress)
				if err != nil {
					if errors.Is(err, store.ErrBlockProcessingMaximumReached) {
						p.logger.Debug("block processing maximum reached", slog.String("hash", hash.String()), slog.String("processed_by", processedBy))
						continue
					} else if errors.Is(err, store.ErrBlockProcessingInProgress) {
						p.logger.Debug("block processing already in progress", slog.String("hash", hash.String()), slog.String("processed_by", processedBy))
						continue
					}

					p.logger.Error("failed to set block processing", slog.String("hash", hash.String()), slog.String("err", err.Error()))
					continue
				}

				p.logger.Info("Sending block request", slog.String("hash", hash.String()))
				msg := wire.NewMsgGetDataSizeHint(1)
				_ = msg.AddInvVect(wire.NewInvVect(wire.InvTypeBlock, hash)) // ignore error at this point
				peer.WriteMsg(msg)

				p.logger.Info("Block request message sent to peer", slog.String("hash", hash.String()), slog.String("peer", peer.String()))
			}
		}
	}()
}

func (p *Processor) StartBlockProcessing() {
	p.waitGroup.Add(1)

	go func() {
		defer p.waitGroup.Done()

		for {
			select {
			case <-p.ctx.Done():
				return
			case blockMsg := <-p.blockProcessCh:
				var err error
				timeStart := time.Now()

				hash := blockMsg.Hash

				p.logger.Info("received block", slog.String("hash", hash.String()))

				err = p.processBlock(blockMsg)
				if err != nil {
					p.logger.Error("block processing failed", slog.String("hash", hash.String()), slog.String("err", err.Error()))
					continue
				}

				storeErr := p.store.MarkBlockAsDone(p.ctx, hash, blockMsg.Size, uint64(len(blockMsg.TransactionHashes)))
				if storeErr != nil {
					p.logger.Error("unable to mark block as processed", slog.String("hash", hash.String()), slog.String("err", storeErr.Error()))
					continue
				}

				timeElapsed := time.Since(timeStart)
				nTxs := len(blockMsg.TransactionHashes)

				// add the total block processing time to the stats
				p.logger.Info("Processed block", slog.String("hash", hash.String()),
					slog.Uint64("height", blockMsg.Height),
					slog.Int("txs", nTxs),
					slog.String("duration", timeElapsed.String()),
					slog.Float64("txs/s", float64(nTxs)/timeElapsed.Seconds()),
				)
			}
		}
	}()
}

func (p *Processor) RegisterTransaction(txHash []byte) {
	select {
	case p.registerTxsChan <- txHash:
	default:
	}
}

func (p *Processor) StartProcessRegisterTxs() {
	p.waitGroup.Add(1)
	txHashes := make([][]byte, 0, p.registerTxsBatchSize)

	ticker := time.NewTicker(p.registerTxsInterval)
	go func() {
		defer p.waitGroup.Done()
		for {
			select {
			case <-p.ctx.Done():
				return
			case txHash := <-p.registerTxsChan:
				txHashes = append(txHashes, txHash)

				if len(txHashes) < p.registerTxsBatchSize {
					continue
				}

				err := p.processTransactions(txHashes[:])
				if err != nil {
					p.logger.Error("Failed to process transactions", slog.String("err", err.Error()))
				}
				txHashes = txHashes[:0]
				ticker.Reset(p.registerTxsInterval)

			case <-ticker.C:
				if len(txHashes) == 0 {
					continue
				}

				err := p.processTransactions(txHashes[:])
				if err != nil {
					p.logger.Error("Failed to process transactions", slog.String("err", err.Error()))
				}
				txHashes = txHashes[:0]
				ticker.Reset(p.registerTxsInterval)
			}
		}
	}()
}

func (p *Processor) processTransactions(txHashes [][]byte) error {
	if len(txHashes) == 0 {
		return nil
	}

	rowsAffected, err := p.store.RegisterTransactions(p.ctx, txHashes)
	if err != nil {
		return fmt.Errorf("failed to register transactions: %v", err)
	}

	if rowsAffected > 0 {
		p.logger.Info("registered tx hashes", slog.Int("hashes", len(txHashes)), slog.Int64("new", rowsAffected))
	}

	minedTxs, err := p.store.GetMinedTransactions(p.ctx, txHashes)
	if err != nil {
		return fmt.Errorf("failed to get mined txs: %v", err)
	}

	if len(minedTxs) == 0 {
		return nil
	}

	p.logger.Info("mined tx hashes", slog.Int("hashes", len(txHashes)), slog.Int("mined", len(minedTxs)))

	minedTxsIncludingMP, err := p.calculateMerklePaths(p.ctx, minedTxs)
	if err != nil {
		return fmt.Errorf("failed to calculate Merkle paths: %v", err)
	}

	err = p.publishMinedTxs(p.ctx, minedTxsIncludingMP)
	if err != nil {
		return fmt.Errorf("failed to publish mined transactions: %v", err)
	}

	p.logger.Info("published mined txs", slog.Int("hashes", len(minedTxsIncludingMP)))

	return nil
}

func (p *Processor) processBlock(blockMsg *bcnet.BlockMessage) (err error) {
	ctx := p.ctx

	var block *blocktx_api.Block
	blockHash := blockMsg.Hash

	ctx, span := tracing.StartTracing(ctx, "processBlock", p.tracingEnabled, p.tracingAttributes...)
	defer func() {
		if span != nil {
			span.SetAttributes(attribute.String("hash", blockHash.String()))
			if block != nil {
				span.SetAttributes(attribute.String("status", block.Status.String()))
			}
		}

		tracing.EndTracing(span, err)
	}()

	p.logger.Info("processing incoming block", slog.String("hash", blockHash.String()), slog.Uint64("height", blockMsg.Height))

	// check if we've already processed that block
	existingBlock, _ := p.store.GetBlock(ctx, blockHash)

	if existingBlock != nil {
		p.logger.Warn("ignoring already existing block", slog.String("hash", blockHash.String()), slog.Uint64("height", blockMsg.Height))
		return nil
	}

	block, err = p.verifyAndInsertBlock(ctx, blockMsg)
	if err != nil {
		return err
	}

	var longestTxs, staleTxs []store.BlockTransaction
	var ok bool

	switch block.Status {
	case blocktx_api.Status_LONGEST:
		longestTxs, ok = p.getRegisteredTransactions(ctx, []*blocktx_api.Block{block})
	case blocktx_api.Status_STALE:
		longestTxs, staleTxs, ok = p.handleStaleBlock(ctx, block)
	case blocktx_api.Status_ORPHANED:
		longestTxs, staleTxs, ok = p.handleOrphans(ctx, block)
	default:
		return ErrUnexpectedBlockStatus
	}

	if !ok {
		// error is already logged in each method above
		return ErrFailedToProcessBlock
	}

	allTxs := append(longestTxs, staleTxs...)

	txsToPublish, err := p.calculateMerklePaths(ctx, allTxs)
	if err != nil {
		return ErrFailedToCalculateMissingMerklePaths
	}

	err = p.publishMinedTxs(ctx, txsToPublish)
	if err != nil {
		return err
	}

	return nil
}

func (p *Processor) verifyAndInsertBlock(ctx context.Context, blockMsg *bcnet.BlockMessage) (incomingBlock *blocktx_api.Block, err error) {
	ctx, span := tracing.StartTracing(ctx, "verifyAndInsertBlock", p.tracingEnabled, p.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	blockHash := blockMsg.Hash

	previousBlockHash := blockMsg.Header.PrevBlock
	merkleRoot := blockMsg.Header.MerkleRoot

	incomingBlock = &blocktx_api.Block{
		Hash:         blockHash[:],
		PreviousHash: previousBlockHash[:],
		MerkleRoot:   merkleRoot[:],
		Height:       blockMsg.Height,
		Chainwork:    calculateChainwork(blockMsg.Header.Bits).String(),
	}

	if p.incomingIsLongest {
		incomingBlock.Status = blocktx_api.Status_LONGEST
	} else {
		err = p.assignBlockStatus(ctx, incomingBlock, previousBlockHash)
		if err != nil {
			p.logger.Error("unable to assign block status", slog.String("hash", blockHash.String()), slog.Uint64("height", incomingBlock.Height), slog.String("err", err.Error()))
			return nil, err
		}
	}

	p.logger.Info("Inserting block", slog.String("hash", blockHash.String()), slog.Uint64("height", incomingBlock.Height), slog.String("status", incomingBlock.Status.String()))

	err = p.insertBlockAndStoreTransactions(ctx, incomingBlock, blockMsg.TransactionHashes, blockMsg.Header.MerkleRoot)
	if err != nil {
		p.logger.Error("unable to insert block and store its transactions", slog.String("hash", blockHash.String()), slog.Uint64("height", incomingBlock.Height), slog.String("err", err.Error()))
		return nil, err
	}

	return incomingBlock, nil
}

func (p *Processor) assignBlockStatus(ctx context.Context, block *blocktx_api.Block, prevBlockHash chainhash.Hash) (err error) {
	ctx, span := tracing.StartTracing(ctx, "assignBlockStatus", p.tracingEnabled, p.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	prevBlock, _ := p.store.GetBlock(ctx, &prevBlockHash)

	if prevBlock == nil {
		// This check is only in case there's a fresh, empty database
		// with no blocks, to mark the first block as the LONGEST chain
		var longestTipExists bool
		longestTipExists, err = p.longestTipExists(ctx)
		if err != nil {
			p.logger.Error("unable to verify the longest tip existence in db", slog.String("hash", getHashStringNoErr(block.Hash)), slog.Uint64("height", block.Height), slog.String("err", err.Error()))
			return err
		}

		// if there's no longest block in the
		// database - mark this block as LONGEST
		// otherwise - it's an orphan
		if !longestTipExists {
			block.Status = blocktx_api.Status_LONGEST
		} else {
			block.Status = blocktx_api.Status_ORPHANED
		}
		return nil
	}

	if prevBlock.Status == blocktx_api.Status_LONGEST {
		var competingBlock *blocktx_api.Block
		competingBlock, err = p.store.GetLongestBlockByHeight(ctx, block.Height)
		if err != nil && !errors.Is(err, store.ErrBlockNotFound) {
			p.logger.Error("unable to get the competing block from db", slog.String("hash", getHashStringNoErr(block.Hash)), slog.Uint64("height", block.Height), slog.String("err", err.Error()))
			return err
		}

		if competingBlock == nil {
			block.Status = blocktx_api.Status_LONGEST
			return nil
		}

		if bytes.Equal(block.Hash, competingBlock.Hash) {
			// this means that another instance is already processing
			// or have processed this block that we're processing here
			// so we can throw an error and finish processing
			err = ErrBlockAlreadyExists
			return err
		}

		block.Status = blocktx_api.Status_STALE
		return nil
	}

	// ORPHANED or STALE
	block.Status = prevBlock.Status

	return nil
}

func (p *Processor) longestTipExists(ctx context.Context) (bool, error) {
	_, err := p.store.GetChainTip(ctx)
	if err != nil && !errors.Is(err, store.ErrBlockNotFound) {
		return false, err
	}

	if errors.Is(err, store.ErrBlockNotFound) {
		return false, nil
	}

	return true, nil
}

func (p *Processor) getRegisteredTransactions(ctx context.Context, blocks []*blocktx_api.Block) (txsToPublish []store.BlockTransaction, ok bool) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "getRegisteredTransactions", p.tracingEnabled, p.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	blockHashes := make([][]byte, len(blocks))
	for i, b := range blocks {
		blockHashes[i] = b.Hash
	}

	txsToPublish, err = p.store.GetRegisteredTxsByBlockHashes(ctx, blockHashes)
	if err != nil {
		block := blocks[len(blocks)-1]
		p.logger.Error("unable to get registered transactions", slog.String("hash", getHashStringNoErr(block.Hash)), slog.Uint64("height", block.Height), slog.String("err", err.Error()))
		return nil, false
	}

	return txsToPublish, true
}

func (p *Processor) insertBlockAndStoreTransactions(ctx context.Context, incomingBlock *blocktx_api.Block, txHashes []*chainhash.Hash, merkleRoot chainhash.Hash) (err error) {
	ctx, span := tracing.StartTracing(ctx, "insertBlockAndStoreTransactions", p.tracingEnabled, append(p.tracingAttributes, attribute.Int("txs", len(txHashes)))...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	_, buildMerkleSpan := tracing.StartTracing(ctx, "BuildMerkleTreeStoreChainHash", p.tracingEnabled, p.tracingAttributes...)
	calculatedMerkleTree := bc.BuildMerkleTreeStoreChainHash(txHashes)
	tracing.EndTracing(buildMerkleSpan, nil)
	if !merkleRoot.IsEqual(calculatedMerkleTree[len(calculatedMerkleTree)-1]) {
		p.logger.Error("merkle root mismatch", slog.String("hash", getHashStringNoErr(incomingBlock.Hash)))
		return err
	}

	blockID, err := p.store.UpsertBlock(ctx, incomingBlock)
	if err != nil {
		p.logger.Error("unable to insert block at given height", slog.String("hash", getHashStringNoErr(incomingBlock.Hash)), slog.Uint64("height", incomingBlock.Height), slog.String("err", err.Error()))
		return err
	}

	if err = p.storeTransactions(ctx, blockID, incomingBlock, txHashes); err != nil {
		p.logger.Error("unable to store transactions from block", slog.String("hash", getHashStringNoErr(incomingBlock.Hash)), slog.String("err", err.Error()))
		return err
	}

	if err = p.handleSkippedBlock(ctx, incomingBlock); err != nil {
		p.logger.Error("unable to store transactions from block", slog.String("hash", getHashStringNoErr(incomingBlock.Hash)), slog.String("err", err.Error()))
		return err
	}

	return nil
}

func (p *Processor) storeTransactions(ctx context.Context, blockID uint64, block *blocktx_api.Block, txHashes []*chainhash.Hash) (err error) {
	ctx, span := tracing.StartTracing(ctx, "storeTransactions", p.tracingEnabled, p.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	txs := make([]store.TxHashWithMerkleTreeIndex, 0, p.transactionStorageBatchSize)

	for txIndex, hash := range txHashes {
		tx := store.TxHashWithMerkleTreeIndex{
			Hash:            hash[:],
			MerkleTreeIndex: int64(txIndex),
		}

		txs = append(txs, tx)
	}

	blockhash, err := chainhash.NewHash(block.Hash)
	if err != nil {
		return errors.Join(ErrFailedToParseBlockHash, fmt.Errorf("block height: %d", block.Height), err)
	}

	totalSize := len(txHashes)

	now := time.Now()

	batchSize := p.transactionStorageBatchSize

	batches := math.Ceil(float64(len(txs)) / float64(batchSize))
	g, ctx := errgroup.WithContext(ctx)

	g.SetLimit(parallellism)

	var txsInserted int64

	finished := make(chan struct{})
	defer func() {
		finished <- struct{}{}
	}()
	go func() {
		step := int64(math.Ceil(float64(len(txs)) / 5))

		showProgress := step
		ticker := time.NewTicker(1 * time.Second)
		for {
			select {
			case <-ticker.C:
				inserted := atomic.LoadInt64(&txsInserted)
				if inserted > showProgress {
					percentage := int64(math.Floor(100 * float64(inserted) / float64(totalSize)))
					p.logger.Info(
						fmt.Sprintf("%d txs out of %d stored", inserted, totalSize),
						slog.Int64("percentage", percentage),
						slog.String("hash", blockhash.String()),
						slog.Uint64("height", block.Height),
						slog.String("duration", time.Since(now).String()),
					)
					showProgress += step
				}
			case <-finished:
				ticker.Stop()
				return
			}
		}
	}()

	for i := 0; i < int(batches); i++ {
		batch := make([]store.TxHashWithMerkleTreeIndex, 0, batchSize)
		if (i+1)*batchSize > len(txs) {
			batch = txs[i*batchSize:]
		} else {
			batch = txs[i*batchSize : (i+1)*batchSize]
		}
		g.Go(func() error {
			insertErr := p.store.InsertBlockTransactions(ctx, blockID, batch)
			if insertErr != nil {
				return errors.Join(ErrFailedToInsertBlockTransactions, insertErr)
			}

			atomic.AddInt64(&txsInserted, int64(len(batch)))
			return nil
		})
	}

	err = g.Wait()
	if err != nil {
		return errors.Join(ErrFailedToInsertBlockTransactions, fmt.Errorf("block height: %d", block.Height), err)
	}

	return nil
}

func (p *Processor) handleStaleBlock(ctx context.Context, block *blocktx_api.Block) (longestTxs, staleTxs []store.BlockTransaction, ok bool) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "handleStaleBlock", p.tracingEnabled, p.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	staleBlocks, err := p.store.GetStaleChainBackFromHash(ctx, block.Hash)
	if err != nil {
		p.logger.Error("unable to get STALE blocks to verify chainwork", slog.String("hash", getHashStringNoErr(block.Hash)), slog.Uint64("height", block.Height), slog.String("err", err.Error()))
		return nil, nil, false
	}

	lowestHeight := block.Height
	if len(staleBlocks) > 0 {
		lowestHeight = staleBlocks[0].Height
	}

	longestBlocks, err := p.store.GetLongestChainFromHeight(ctx, lowestHeight)
	if err != nil {
		p.logger.Error("unable to get LONGEST blocks to verify chainwork", slog.String("hash", getHashStringNoErr(block.Hash)), slog.Uint64("height", block.Height), slog.String("err", err.Error()))
		return nil, nil, false
	}

	staleChainwork := sumChainwork(staleBlocks)
	longestChainwork := sumChainwork(longestBlocks)

	if longestChainwork.Cmp(staleChainwork) < 0 {
		p.logger.Info("chain reorg detected", slog.String("hash", getHashStringNoErr(block.Hash)), slog.Uint64("height", block.Height))

		longestTxs, staleTxs, err = p.performReorg(ctx, staleBlocks, longestBlocks)
		if err != nil {
			p.logger.Error("unable to perform reorg", slog.String("hash", getHashStringNoErr(block.Hash)), slog.Uint64("height", block.Height), slog.String("err", err.Error()))
			return nil, nil, false
		}
		return longestTxs, staleTxs, true
	}

	return nil, nil, true
}

func (p *Processor) handleSkippedBlock(ctx context.Context, block *blocktx_api.Block) (err error) {
	ctx, span := tracing.StartTracing(ctx, "handleSkippedBlock", p.tracingEnabled, p.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	staleBlocks, err := p.store.GetStaleChainForwardFromHash(ctx, block.Hash)
	if err != nil {
		p.logger.Error("unable to get STALE blocks to verify chainwork", slog.String("hash", getHashStringNoErr(block.Hash)), slog.Uint64("height", block.Height), slog.String("err", err.Error()))
		return err
	}

	_, _, err = p.performReorg(ctx, staleBlocks, []*blocktx_api.Block{})
	if err != nil {
		p.logger.Error("unable to fix gap blocks", slog.String("hash", getHashStringNoErr(block.Hash)), slog.Uint64("height", block.Height), slog.String("err", err.Error()))
		return err
	}
	return nil
}

func (p *Processor) performReorg(ctx context.Context, staleBlocks []*blocktx_api.Block, longestBlocks []*blocktx_api.Block) (longestTxs, staleTxs []store.BlockTransaction, err error) {
	ctx, span := tracing.StartTracing(ctx, "performReorg", p.tracingEnabled, p.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	staleHashes := make([][]byte, len(staleBlocks))
	longestHashes := make([][]byte, len(longestBlocks))

	blockStatusUpdates := make([]store.BlockStatusUpdate, len(longestBlocks)+len(staleBlocks))

	for i, b := range longestBlocks {
		longestHashes[i] = b.Hash

		b.Status = blocktx_api.Status_STALE
		update := store.BlockStatusUpdate{Hash: b.Hash, Status: b.Status}
		blockStatusUpdates[i] = update
	}

	for i, b := range staleBlocks {
		staleHashes[i] = b.Hash

		b.Status = blocktx_api.Status_LONGEST
		update := store.BlockStatusUpdate{Hash: b.Hash, Status: b.Status}
		blockStatusUpdates[i+len(longestBlocks)] = update
	}

	err = p.store.UpdateBlocksStatuses(ctx, blockStatusUpdates)
	if err != nil {
		return nil, nil, err
	}

	p.logger.Info("reorg performed successfully")

	// now the previously stale chain is the longest,
	// so longestTxs are from previously stale block hashes
	longestTxs, err = p.store.GetRegisteredTxsByBlockHashes(ctx, staleHashes)
	if err != nil {
		return nil, nil, err
	}

	// now the previously longest chain is stale,
	// so staleTxs are from previously longest block hashes
	staleTxs, err = p.store.GetRegisteredTxsByBlockHashes(ctx, longestHashes)
	if err != nil {
		return nil, nil, err
	}

	staleTxs = exclusiveRightTxs(longestTxs, staleTxs)

	return longestTxs, staleTxs, nil
}

func (p *Processor) handleOrphans(ctx context.Context, block *blocktx_api.Block) (longestTxs, staleTxs []store.BlockTransaction, ok bool) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "handleOrphans", p.tracingEnabled, p.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	orphans, ancestor, err := p.store.GetOrphansBackToNonOrphanAncestor(ctx, block.Hash)
	if err != nil {
		p.logger.Error("unable to get ORPHANED blocks", slog.String("hash", getHashStringNoErr(block.Hash)), slog.Uint64("height", block.Height), slog.String("err", err.Error()))
		return nil, nil, false
	}

	if ancestor == nil || len(orphans) == 0 {
		return nil, nil, true
	}

	p.logger.Info("orphaned chain found", slog.String("hash", getHashStringNoErr(block.Hash)), slog.Uint64("height", block.Height), slog.String("status", block.Status.String()))

	if ancestor.Status == blocktx_api.Status_STALE {
		ok = p.acceptIntoChain(ctx, orphans, ancestor.Status)
		if !ok {
			return nil, nil, false
		}

		block.Status = blocktx_api.Status_STALE
		return p.handleStaleBlock(ctx, block)
	}

	if ancestor.Status == blocktx_api.Status_LONGEST {
		// If there is competing block at the height of
		// the first orphan, then we need to mark them
		// all as stale and recheck for reorg.
		//
		// If there's no competing block at the height
		// of the first orphan, then we can assume that
		// there's no competing chain at all.

		var competingBlock *blocktx_api.Block
		competingBlock, err = p.store.GetLongestBlockByHeight(ctx, orphans[0].Height)
		if err != nil && !errors.Is(err, store.ErrBlockNotFound) {
			p.logger.Error("unable to get competing block when handling orphans", slog.String("hash", getHashStringNoErr(block.Hash)), slog.Uint64("height", block.Height), slog.String("err", err.Error()))
			return nil, nil, false
		}

		if competingBlock != nil && !bytes.Equal(competingBlock.Hash, orphans[0].Hash) {
			ok = p.acceptIntoChain(ctx, orphans, blocktx_api.Status_STALE)
			if !ok {
				return nil, nil, false
			}

			block.Status = blocktx_api.Status_STALE
			return p.handleStaleBlock(ctx, block)
		}

		ok = p.acceptIntoChain(ctx, orphans, ancestor.Status) // LONGEST
		if !ok {
			return nil, nil, false
		}

		p.logger.Info("orphaned chain accepted into LONGEST chain", slog.String("hash", getHashStringNoErr(block.Hash)), slog.Uint64("height", block.Height))
		longestTxs, ok = p.getRegisteredTransactions(ctx, orphans)
		return longestTxs, nil, ok
	}

	return nil, nil, true
}

func (p *Processor) acceptIntoChain(ctx context.Context, blocks []*blocktx_api.Block, chain blocktx_api.Status) (ok bool) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "acceptIntoChain", p.tracingEnabled, p.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	blockStatusUpdates := make([]store.BlockStatusUpdate, len(blocks))

	for i, b := range blocks {
		b.Status = chain
		blockStatusUpdates[i] = store.BlockStatusUpdate{
			Hash:   b.Hash,
			Status: b.Status,
		}
	}

	tip := blocks[len(blocks)-1]

	err = p.store.UpdateBlocksStatuses(ctx, blockStatusUpdates)
	if err != nil {
		p.logger.Error("unable to accept blocks into chain", slog.String("hash", getHashStringNoErr(tip.Hash)), slog.Uint64("height", tip.Height), slog.String("chain", chain.String()), slog.String("err", err.Error()))
		return false
	}

	p.logger.Info("blocks successfully accepted into chain", slog.String("hash", getHashStringNoErr(tip.Hash)), slog.Uint64("height", tip.Height), slog.String("chain", chain.String()))
	return true
}

func (p *Processor) publishMinedTxs(ctx context.Context, txs []store.BlockTransactionWithMerklePath) error {
	var publishErr error
	_, span := tracing.StartTracing(ctx, "publishMinedTxs", p.tracingEnabled, p.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, publishErr)
	}()

	msg := &blocktx_api.TransactionBlocks{
		TransactionBlocks: make([]*blocktx_api.TransactionBlock, 0, p.publishMinedMessageSize),
	}

	for _, tx := range txs {
		txBlock := &blocktx_api.TransactionBlock{
			BlockHash:       tx.BlockHash,
			BlockHeight:     tx.BlockHeight,
			TransactionHash: tx.TxHash,
			MerklePath:      tx.MerklePath,
			BlockStatus:     tx.BlockStatus,
		}

		msg.TransactionBlocks = append(msg.TransactionBlocks, txBlock)

		if len(msg.TransactionBlocks) >= p.publishMinedMessageSize {
			err := p.mqClient.PublishMarshalCore(mq.MinedTxsTopic, msg)
			if err != nil {
				p.logger.Error("Failed to publish mined txs", slog.String("blockHash", getHashStringNoErr(tx.BlockHash)), slog.Uint64("height", tx.BlockHeight), slog.String("err", err.Error()))
				publishErr = errors.Join(publishErr, err)
			}

			msg = &blocktx_api.TransactionBlocks{
				TransactionBlocks: make([]*blocktx_api.TransactionBlock, 0, p.publishMinedMessageSize),
			}
		}
	}

	if len(msg.TransactionBlocks) > 0 {
		err := p.mqClient.PublishMarshalCore(mq.MinedTxsTopic, msg)
		if err != nil {
			p.logger.Error("failed to publish mined txs", slog.String("err", err.Error()))
			publishErr = errors.Join(publishErr, err)
		}
	}

	return publishErr
}

func (p *Processor) calculateMerklePaths(ctx context.Context, txs []store.BlockTransaction) (updatedTxs []store.BlockTransactionWithMerklePath, err error) {
	ctx, span := tracing.StartTracing(ctx, "calculateMerklePaths", p.tracingEnabled, p.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	// gather all transactions with missing merkle paths for each block in a map
	// to avoid getting all transaction from the same block multiple times
	blockTxsMap := make(map[string][]store.BlockTransactionWithMerklePath)

	for _, tx := range txs {
		blockTransactionWithMerklePath := store.BlockTransactionWithMerklePath{
			BlockTransaction: store.BlockTransaction{
				TxHash:          tx.TxHash,
				BlockHash:       tx.BlockHash,
				BlockHeight:     tx.BlockHeight,
				MerkleTreeIndex: tx.MerkleTreeIndex,
				BlockStatus:     tx.BlockStatus,
				MerkleRoot:      tx.MerkleRoot,
			},
		}

		blockTxsMap[hex.EncodeToString(tx.BlockHash)] = append(blockTxsMap[hex.EncodeToString(tx.BlockHash)], blockTransactionWithMerklePath)
	}

	for bh, blockTxs := range blockTxsMap {
		blockHash, err := hex.DecodeString(bh)
		if err != nil {
			return nil, err
		}

		txHashes, err := p.store.GetBlockTransactionsHashes(ctx, blockHash)
		if err != nil {
			return nil, errors.Join(ErrFailedToGetBlockTransactions, fmt.Errorf("block hash %s", getHashStringNoErr(blockHash)), err)
		}

		if len(txHashes) == 0 {
			continue
		}

		merkleTree := bc.BuildMerkleTreeStoreChainHash(txHashes)

		for _, tx := range blockTxs {
			merkleIndex := tx.MerkleTreeIndex

			txHash, err := chainhash.NewHash(tx.TxHash)
			const merkleTreeIndex = "merkle tree index"
			const blockHash = "block hash"
			if err != nil {
				p.logger.Error("Failed to create chain hash", slog.Int64(merkleTreeIndex, merkleIndex), slog.String(blockHash, bh), slog.String("err", err.Error()))
				continue
			}
			txID := txHash.String()

			txIndex := uint64(merkleIndex)
			if merkleIndex < 0 {
				p.logger.Warn("missing merkle tree index for transaction", slog.String("hash", getHashStringNoErr(tx.TxHash)))
				continue
			}
			bump, err := bc.NewBUMPFromMerkleTreeAndIndex(tx.BlockHeight, merkleTree, txIndex)
			if err != nil {
				p.logger.Error("Failed to create bump from Merkle tree and index", slog.String("hash", txID), slog.Int64(merkleTreeIndex, merkleIndex), slog.String(blockHash, bh), slog.String("err", err.Error()))
				continue
			}

			bumpHex, err := bump.String()
			if err != nil {
				p.logger.Error("Failed to create bump string", slog.String("hash", txID), slog.Int64(merkleTreeIndex, merkleIndex), slog.String(blockHash, bh), slog.String("err", err.Error()))
				continue
			}

			path, err := sdkTx.NewMerklePathFromHex(bumpHex)
			if err != nil {
				p.logger.Error("Failed to create Merkle path from bump", slog.String("hash", txID), slog.Int64(merkleTreeIndex, merkleIndex), slog.String(blockHash, bh), slog.String("err", err.Error()))
				continue
			}

			root, err := path.ComputeRootHex(&txID)
			if err != nil {
				p.logger.Error("Failed to compute root for tx", slog.String("hash", txID), slog.Int64(merkleTreeIndex, merkleIndex), slog.String(blockHash, bh), slog.String("err", err.Error()))
				continue
			}

			merkleRoot := tx.GetMerkleRootString()
			if root != merkleRoot {
				p.logger.Error("Comparison of Merkle roots failed", slog.String("calc root", root), slog.String("block root", merkleRoot), slog.String("hash", txID), slog.Int64(merkleTreeIndex, merkleIndex), slog.String(blockHash, bh))
				continue
			}

			tx.MerklePath = bumpHex
			updatedTxs = append(updatedTxs, tx)
		}
	}

	return updatedTxs, nil
}

func (p *Processor) Shutdown() {
	p.cancelAll()
	p.waitGroup.Wait()
}
