package blocktx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/libsv/go-bc"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/bitcoin-sv/arc/internal/tracing"
)

var (
	ErrFailedToSubscribeToTopic        = errors.New("failed to subscribe to register topic")
	ErrFailedToCreateBUMP              = errors.New("failed to create new bump for tx hash from merkle tree and index")
	ErrFailedToGetStringFromBUMPHex    = errors.New("failed to get string from bump for tx hash")
	ErrFailedToInsertBlockTransactions = errors.New("failed to insert block transactions")
)

const (
	transactionStoringBatchsizeDefault = 8192 // power of 2 for easier memory allocation
	maxRequestBlocks                   = 10
	maxBlocksInProgress                = 1
	registerTxsIntervalDefault         = time.Second * 10
	registerRequestTxsIntervalDefault  = time.Second * 5
	registerTxsBatchSizeDefault        = 100
	registerRequestTxBatchSizeDefault  = 100
	waitForBlockProcessing             = 5 * time.Minute
)

type Processor struct {
	hostname                    string
	blockRequestCh              chan BlockRequest
	blockProcessCh              chan *p2p.BlockMessage
	store                       store.BlocktxStore
	logger                      *slog.Logger
	transactionStorageBatchSize int
	dataRetentionDays           int
	mqClient                    MessageQueueClient
	registerTxsChan             chan []byte
	requestTxChannel            chan []byte
	registerTxsInterval         time.Duration
	registerRequestTxsInterval  time.Duration
	registerTxsBatchSize        int
	registerRequestTxsBatchSize int
	tracingEnabled              bool
	tracingAttributes           []attribute.KeyValue
	processGuardsMap            sync.Map

	waitGroup *sync.WaitGroup
	cancelAll context.CancelFunc
	ctx       context.Context
}

func NewProcessor(
	logger *slog.Logger,
	storeI store.BlocktxStore,
	blockRequestCh chan BlockRequest,
	blockProcessCh chan *p2p.BlockMessage,
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
		registerRequestTxsBatchSize: registerRequestTxBatchSizeDefault,
		hostname:                    hostname,
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
	err := p.mqClient.Subscribe(RegisterTxTopic, func(msg []byte) error {
		p.registerTxsChan <- msg
		return nil
	})
	if err != nil {
		return errors.Join(ErrFailedToSubscribeToTopic, fmt.Errorf("topic: %s", RegisterTxTopic), err)
	}

	err = p.mqClient.Subscribe(RequestTxTopic, func(msg []byte) error {
		p.requestTxChannel <- msg
		return nil
	})
	if err != nil {
		return errors.Join(ErrFailedToSubscribeToTopic, fmt.Errorf("topic: %s", RequestTxTopic), err)
	}

	p.StartBlockRequesting()
	p.StartBlockProcessing()
	p.StartProcessRegisterTxs()
	p.StartProcessRequestTxs()

	return nil
}

func (p *Processor) StartBlockRequesting() {
	p.waitGroup.Add(1)

	waitUntilFree := func(ctx context.Context) bool {
		t := time.NewTicker(time.Second)

		for {
			bhs, err := p.store.GetBlockHashesProcessingInProgress(p.ctx, p.hostname)
			if err != nil {
				p.logger.Error("failed to get block hashes where processing in progress", slog.String("err", err.Error()))
			}

			if len(bhs) < maxBlocksInProgress && err == nil {
				return true
			}

			select {
			case <-ctx.Done():
				return false

			case <-t.C:
			}
		}
	}

	go func() {
		defer p.waitGroup.Done()
		for {
			select {
			case <-p.ctx.Done():
				return
			case req := <-p.blockRequestCh:
				hash := req.Hash
				peer := req.Peer

				if ok := waitUntilFree(p.ctx); !ok {
					continue
				}

				// lock block for the current instance to process
				processedBy, err := p.store.SetBlockProcessing(p.ctx, hash, p.hostname)
				if err != nil {
					// block is already being processed by another blocktx instance
					if errors.Is(err, store.ErrBlockProcessingDuplicateKey) {
						p.logger.Debug("block processing already in progress", slog.String("hash", hash.String()), slog.String("processed_by", processedBy))
						continue
					}

					p.logger.Error("failed to set block processing", slog.String("hash", hash.String()), slog.String("err", err.Error()))
					continue
				}

				msg := wire.NewMsgGetData()
				_ = msg.AddInvVect(wire.NewInvVect(wire.InvTypeBlock, hash)) // ignore error at this point

				p.logger.Info("Sending block request", slog.String("hash", hash.String()))
				if err = peer.WriteMsg(msg); err != nil {
					p.logger.Error("failed to write block request message to peer", slog.String("hash", hash.String()), slog.String("err", err.Error()))
					p.unlockBlock(p.ctx, hash)

					continue
				}

				p.startBlockProcessGuard(p.ctx, hash)
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
				blockhash := blockMsg.Header.BlockHash()

				defer p.stopBlockProcessGuard(&blockhash) // release guardian at the end

				p.logger.Info("received block", slog.String("hash", blockhash.String()))
				err := p.processBlock(blockMsg)
				if err != nil {
					p.logger.Error("block processing failed", slog.String("hash", blockhash.String()), slog.String("err", err.Error()))
					p.unlockBlock(p.ctx, &blockhash)
				}
			}
		}
	}()
}

func (p *Processor) startBlockProcessGuard(ctx context.Context, hash *chainhash.Hash) {
	p.waitGroup.Add(1)

	execCtx, stopFn := context.WithCancel(ctx)
	p.processGuardsMap.Store(hash, stopFn)

	go func() {
		defer p.waitGroup.Done()
		defer p.processGuardsMap.Delete(*hash)

		select {
		case <-execCtx.Done():
			// we may do nothing here:
			// 1. block processing is completed, or
			// 2. processor is shutting down – all unprocessed blocks are released in the Shutdown func
			return

		case <-time.After(waitForBlockProcessing):
			// check if block was processed successfully
			block, _ := p.store.GetBlock(execCtx, hash)

			if block != nil && block.Processed {
				return // success
			}

			p.logger.Warn(fmt.Sprintf("block was not processed after %v. Unlock the block to be processed later", waitForBlockProcessing), slog.String("hash", hash.String()))
			p.unlockBlock(execCtx, hash)
		}
	}()
}

func (p *Processor) stopBlockProcessGuard(hash *chainhash.Hash) {
	stopFn, found := p.processGuardsMap.Load(*hash)
	if found {
		stopFn.(context.CancelFunc)()
	}
}

// unlock block for future processing
func (p *Processor) unlockBlock(ctx context.Context, hash *chainhash.Hash) {
	// use closures for retries
	unlockFn := func() error {
		_, err := p.store.DelBlockProcessing(ctx, hash, p.hostname)
		if errors.Is(err, store.ErrBlockNotFound) {
			return nil // block is already unlocked
		}

		return err
	}

	var bo backoff.BackOff
	bo = backoff.NewConstantBackOff(100 * time.Millisecond)
	bo = backoff.WithContext(bo, ctx)
	bo = backoff.WithMaxRetries(bo, 5)

	if unlockErr := backoff.Retry(unlockFn, bo); unlockErr != nil {
		p.logger.ErrorContext(ctx, "failed to delete block processing", slog.String("hash", hash.String()), slog.String("err", unlockErr.Error()))
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

				p.registerTransactions(txHashes[:])
				txHashes = txHashes[:0]
				ticker.Reset(p.registerTxsInterval)

			case <-ticker.C:
				if len(txHashes) == 0 {
					continue
				}

				p.registerTransactions(txHashes[:])
				txHashes = txHashes[:0]
				ticker.Reset(p.registerTxsInterval)
			}
		}
	}()
}

func (p *Processor) StartProcessRequestTxs() {
	p.waitGroup.Add(1)

	txHashes := make([]*chainhash.Hash, 0, p.registerRequestTxsBatchSize)

	ticker := time.NewTicker(p.registerRequestTxsInterval)

	go func() {
		defer p.waitGroup.Done()

		for {
			select {
			case <-p.ctx.Done():
				return
			case txHash := <-p.requestTxChannel:
				tx, err := chainhash.NewHash(txHash)
				if err != nil {
					p.logger.Error("Failed to create hash from byte array", slog.String("err", err.Error()))
					continue
				}

				txHashes = append(txHashes, tx)

				if len(txHashes) < p.registerRequestTxsBatchSize || len(txHashes) == 0 {
					continue
				}

				err = p.publishMinedTxs(txHashes)
				if err != nil {
					p.logger.Error("failed to publish mined txs", slog.String("err", err.Error()))
					continue // retry, don't clear the txHashes slice
				}

				txHashes = make([]*chainhash.Hash, 0, p.registerRequestTxsBatchSize)
				ticker.Reset(p.registerRequestTxsInterval)

			case <-ticker.C:
				if len(txHashes) == 0 {
					continue
				}

				err := p.publishMinedTxs(txHashes)
				if err != nil {
					p.logger.Error("failed to publish mined txs", slog.String("err", err.Error()))
					ticker.Reset(p.registerRequestTxsInterval)
					continue // retry, don't clear the txHashes slice
				}

				txHashes = make([]*chainhash.Hash, 0, p.registerRequestTxsBatchSize)
				ticker.Reset(p.registerRequestTxsInterval)
			}
		}
	}()
}

func (p *Processor) publishMinedTxs(txHashes []*chainhash.Hash) error {
	minedTxs, err := p.store.GetMinedTransactions(p.ctx, txHashes)
	if err != nil {
		return fmt.Errorf("failed to get mined transactions: %v", err)
	}

	for _, minedTx := range minedTxs {
		txBlock := &blocktx_api.TransactionBlock{
			TransactionHash: minedTx.TxHash,
			BlockHash:       minedTx.BlockHash,
			BlockHeight:     minedTx.BlockHeight,
			MerklePath:      minedTx.MerklePath,
		}
		err = p.mqClient.PublishMarshal(MinedTxsTopic, txBlock)
	}

	if err != nil {
		return fmt.Errorf("failed to publish mined transactions: %v", err)
	}

	return nil
}

func (p *Processor) registerTransactions(txHashes [][]byte) {
	updatedTxs, err := p.store.RegisterTransactions(p.ctx, txHashes)
	if err != nil {
		p.logger.Error("failed to register transactions", slog.String("err", err.Error()))
	}

	if len(updatedTxs) > 0 {
		err = p.publishMinedTxs(updatedTxs)
		if err != nil {
			p.logger.Error("failed to publish mined txs", slog.String("err", err.Error()))
		}
	}
}

func (p *Processor) buildMerkleTreeStoreChainHash(ctx context.Context, txids []*chainhash.Hash) []*chainhash.Hash {
	_, span := tracing.StartTracing(ctx, "buildMerkleTreeStoreChainHash", p.tracingEnabled, p.tracingAttributes...)
	defer tracing.EndTracing(span)

	return bc.BuildMerkleTreeStoreChainHash(txids)
}

func (p *Processor) processBlock(msg *p2p.BlockMessage) error {
	ctx := p.ctx

	ctx, span := tracing.StartTracing(ctx, "HandleBlock", p.tracingEnabled, p.tracingAttributes...)
	defer tracing.EndTracing(span)

	timeStart := time.Now()

	blockHash := msg.Header.BlockHash()
	previousBlockHash := msg.Header.PrevBlock
	merkleRoot := msg.Header.MerkleRoot

	// don't process block that was already processed
	existingBlock, _ := p.store.GetBlock(ctx, &blockHash)
	if existingBlock != nil && existingBlock.Processed {
		return nil
	}

	prevBlock, err := p.getPrevBlock(ctx, &previousBlockHash)
	if err != nil {
		p.logger.Error("unable to get previous block from db", slog.String("hash", blockHash.String()), slog.Uint64("height", msg.Height), slog.String("prevHash", previousBlockHash.String()), slog.String("err", err.Error()))
		return err
	}

	longestTipExists := true
	if prevBlock == nil {
		// This check is only in case there's a fresh, empty database
		// with no blocks, to mark the first block as the LONGEST chain
		longestTipExists, err = p.longestTipExists(ctx)
		if err != nil {
			p.logger.Error("unable to verify the longest tip existance in db", slog.String("hash", blockHash.String()), slog.Uint64("height", msg.Height), slog.String("err", err.Error()))
			return err
		}
	}

	incomingBlock := createBlock(msg, prevBlock, longestTipExists)

	competing, err := p.competingChainsExist(ctx, incomingBlock)
	if err != nil {
		p.logger.Error("unable to check for competing chains", slog.String("hash", blockHash.String()), slog.Uint64("height", msg.Height), slog.String("err", err.Error()))
		return err
	}

	if competing {
		p.logger.Info("Competing blocks found", slog.String("incoming block hash", blockHash.String()), slog.Uint64("height", incomingBlock.Height))

		hasGreatestChainwork, err := p.hasGreatestChainwork(ctx, incomingBlock)
		if err != nil {
			p.logger.Error("unable to get the chain tip to verify chainwork", slog.String("hash", blockHash.String()), slog.Uint64("height", incomingBlock.Height), slog.String("err", err.Error()))
			return err
		}

		// find competing chains back to the common ancestor
		// get all registered transactions
		// prepare msg with competing blocks
		incomingBlock.Status = blocktx_api.Status_STALE

		if hasGreatestChainwork {
			p.logger.Info("reorg detected - updating blocks", slog.String("hash", blockHash.String()), slog.Uint64("height", incomingBlock.Height))

			incomingBlock.Status = blocktx_api.Status_LONGEST

			err := p.performReorg(ctx, incomingBlock)
			if err != nil {
				p.logger.Error("unable to perform reorg", slog.String("hash", blockHash.String()), slog.Uint64("height", incomingBlock.Height), slog.String("err", err.Error()))
				return err
			}
		}
	}

	p.logger.Info("Upserting block", slog.String("hash", blockHash.String()), slog.Uint64("height", incomingBlock.Height), slog.String("status", incomingBlock.Status.String()))

	blockID, err := p.store.UpsertBlock(ctx, incomingBlock)
	if err != nil {
		p.logger.Error("unable to upsert block at given height", slog.String("hash", blockHash.String()), slog.Uint64("height", msg.Height), slog.String("err", err.Error()))
		return err
	}

	calculatedMerkleTree := p.buildMerkleTreeStoreChainHash(ctx, msg.TransactionHashes)

	if !merkleRoot.IsEqual(calculatedMerkleTree[len(calculatedMerkleTree)-1]) {
		p.logger.Error("merkle root mismatch", slog.String("hash", blockHash.String()))
		return err
	}

	if err = p.markTransactionsAsMined(ctx, blockID, calculatedMerkleTree, msg.Height, &blockHash); err != nil {
		p.logger.Error("unable to mark block as mined", slog.String("hash", blockHash.String()), slog.String("err", err.Error()))
		return err
	}

	if err = p.store.MarkBlockAsDone(ctx, &blockHash, msg.Size, uint64(len(msg.TransactionHashes))); err != nil {
		p.logger.Error("unable to mark block as processed", slog.String("hash", blockHash.String()), slog.String("err", err.Error()))
		return err
	}

	// add the total block processing time to the stats
	p.logger.Info("Processed block", slog.String("hash", blockHash.String()), slog.Int("txs", len(msg.TransactionHashes)), slog.String("duration", time.Since(timeStart).String()))

	return nil
}

func (p *Processor) getPrevBlock(ctx context.Context, prevHash *chainhash.Hash) (*blocktx_api.Block, error) {
	prevBlock, err := p.store.GetBlock(ctx, prevHash)
	if err != nil && !errors.Is(err, store.ErrBlockNotFound) {
		return nil, err
	}

	return prevBlock, nil
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

func (p *Processor) competingChainsExist(ctx context.Context, block *blocktx_api.Block) (bool, error) {
	if block.Status == blocktx_api.Status_ORPHANED {
		return false, nil
	}

	if block.Status == blocktx_api.Status_LONGEST {
		competingBlock, err := p.store.GetBlockByHeight(ctx, block.Height, blocktx_api.Status_LONGEST)
		if err != nil && !errors.Is(err, store.ErrBlockNotFound) {
			return false, err
		}

		if competingBlock != nil && !bytes.Equal(competingBlock.Hash, block.Hash) {
			return true, nil
		}

		return false, nil
	}

	// If STALE status
	return true, nil
}

func (p *Processor) hasGreatestChainwork(ctx context.Context, incomingBlock *blocktx_api.Block) (bool, error) {
	tip, err := p.store.GetChainTip(ctx)
	if err != nil && !errors.Is(err, store.ErrBlockNotFound) {
		return false, err
	}

	// this can happen only in case the blocks table is empty
	if tip == nil {
		return true, nil
	}

	tipChainWork := new(big.Int)
	tipChainWork.SetString(tip.Chainwork, 10)

	incomingBlockChainwork := new(big.Int)
	incomingBlockChainwork.SetString(incomingBlock.Chainwork, 10)

	return tipChainWork.Cmp(incomingBlockChainwork) < 0, nil
}

func (p *Processor) performReorg(ctx context.Context, incomingBlock *blocktx_api.Block) error {
	staleBlocks, err := p.store.GetStaleChainBackFromHash(ctx, incomingBlock.PreviousHash)
	if err != nil {
		return err
	}

	lowestHeight := incomingBlock.Height
	if len(staleBlocks) > 0 {
		lowestHeight = getLowestHeight(staleBlocks)
	}

	longestBlocks, err := p.store.GetLongestChainFromHeight(ctx, lowestHeight)
	if err != nil {
		return err
	}

	blockStatusUpdates := make([]store.BlockStatusUpdate, 0)

	for _, b := range staleBlocks {
		update := store.BlockStatusUpdate{Hash: b.Hash, Status: blocktx_api.Status_LONGEST}
		blockStatusUpdates = append(blockStatusUpdates, update)
	}

	for _, b := range longestBlocks {
		update := store.BlockStatusUpdate{Hash: b.Hash, Status: blocktx_api.Status_STALE}
		blockStatusUpdates = append(blockStatusUpdates, update)
	}

	err = p.store.UpdateBlocksStatuses(ctx, blockStatusUpdates)
	return err
}

func (p *Processor) markTransactionsAsMined(ctx context.Context, blockID uint64, merkleTree []*chainhash.Hash, blockHeight uint64, blockhash *chainhash.Hash) error {
	ctx, span := tracing.StartTracing(ctx, "markTransactionsAsMined", p.tracingEnabled, p.tracingAttributes...)
	defer tracing.EndTracing(span)

	txs := make([]store.TxWithMerklePath, 0, p.transactionStorageBatchSize)
	leaves := merkleTree[:(len(merkleTree)+1)/2]

	var totalSize int
	for totalSize = 1; totalSize < len(leaves); totalSize++ {
		if leaves[totalSize] == nil {
			// Everything to the right of the first nil will also be nil, as this is just padding upto the next PoT.
			break
		}
	}

	progress := progressIndices(totalSize, 5)
	now := time.Now()

	var iterateMerkleTree trace.Span
	ctx, iterateMerkleTree = tracing.StartTracing(ctx, "iterateMerkleTree", p.tracingEnabled, p.tracingAttributes...)

	for txIndex, hash := range leaves {
		// Everything to the right of the first nil will also be nil, as this is just padding upto the next PoT.
		if hash == nil {
			break
		}

		bump, err := bc.NewBUMPFromMerkleTreeAndIndex(blockHeight, merkleTree, uint64(txIndex))
		if err != nil {
			return errors.Join(ErrFailedToCreateBUMP, err)
		}

		bumpHex, err := bump.String()
		if err != nil {
			return errors.Join(ErrFailedToGetStringFromBUMPHex, err)
		}

		txs = append(txs, store.TxWithMerklePath{
			Hash:       hash[:],
			MerklePath: bumpHex,
		})

		if (txIndex+1)%p.transactionStorageBatchSize == 0 {
			updateResp, err := p.store.UpsertBlockTransactions(ctx, blockID, txs)
			if err != nil {
				return errors.Join(ErrFailedToInsertBlockTransactions, err)
			}
			// free up memory
			txs = txs[:0]

			for _, updResp := range updateResp {
				txBlock := &blocktx_api.TransactionBlock{
					TransactionHash: updResp.Hash[:],
					BlockHash:       blockhash[:],
					BlockHeight:     blockHeight,
					MerklePath:      updResp.MerklePath,
				}
				err = p.mqClient.PublishMarshal(MinedTxsTopic, txBlock)
				if err != nil {
					p.logger.Error("failed to publish mined txs", slog.String("hash", blockhash.String()), slog.Uint64("height", blockHeight), slog.String("err", err.Error()))
				}
			}
		}

		if percentage, found := progress[txIndex+1]; found {
			if totalSize > 0 {
				p.logger.Info(fmt.Sprintf("%d txs out of %d marked as mined", txIndex+1, totalSize), slog.Int("percentage", percentage), slog.String("hash", blockhash.String()), slog.Uint64("height", blockHeight), slog.String("duration", time.Since(now).String()))
			}
		}
	}

	tracing.EndTracing(iterateMerkleTree)

	// update all remaining transactions
	updateResp, err := p.store.UpsertBlockTransactions(ctx, blockID, txs)
	if err != nil {
		return errors.Join(ErrFailedToInsertBlockTransactions, fmt.Errorf("block height: %d", blockHeight), err)
	}

	for _, updResp := range updateResp {
		txBlock := &blocktx_api.TransactionBlock{
			TransactionHash: updResp.Hash[:],
			BlockHash:       blockhash[:],
			BlockHeight:     blockHeight,
			MerklePath:      updResp.MerklePath,
		}
		err = p.mqClient.PublishMarshal(MinedTxsTopic, txBlock)
		if err != nil {
			p.logger.Error("failed to publish mined txs", slog.String("hash", blockhash.String()), slog.Uint64("height", blockHeight), slog.String("err", err.Error()))
		}
	}

	return nil
}

func (p *Processor) Shutdown() {
	p.cancelAll()
	p.waitGroup.Wait()

	// unlock unprocessed blocks
	bhs, err := p.store.GetBlockHashesProcessingInProgress(context.Background(), p.hostname)
	if err != nil {
		p.logger.Error("reading unprocessing blocks on shutdown failed", slog.Any("err", err))
		return
	}

	for _, bh := range bhs {
		_, err := p.store.DelBlockProcessing(context.Background(), bh, p.hostname)
		if err != nil {
			p.logger.Error("unlocking unprocessed block on shutdown failed", slog.String("hash", bh.String()), slog.Any("err", err))
		}
	}
}

// GetBlockRequestCh is for testing purposes only
func (p *Processor) GetBlockRequestCh() chan BlockRequest {
	return p.blockRequestCh
}
