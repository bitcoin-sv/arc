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
	ErrFailedToParseBlockHash          = errors.New("failed to parse block hash")
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

				msg := wire.NewMsgGetDataSizeHint(1)
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
				var err error
				blockHash := blockMsg.Header.BlockHash()
				timeStart := time.Now()

				p.logger.Info("received block", slog.String("hash", blockHash.String()))

				err = p.processBlock(blockMsg)
				if err != nil {
					p.logger.Error("block processing failed", slog.String("hash", blockHash.String()), slog.String("err", err.Error()))
					p.unlockBlock(p.ctx, &blockHash)
					p.stopBlockProcessGuard(&blockHash) // release guardian
					continue
				}

				storeErr := p.store.MarkBlockAsDone(p.ctx, &blockHash, blockMsg.Size, uint64(len(blockMsg.TransactionHashes)))
				if storeErr != nil {
					p.logger.Error("unable to mark block as processed", slog.String("hash", blockHash.String()), slog.String("err", storeErr.Error()))
					p.unlockBlock(p.ctx, &blockHash)
					p.stopBlockProcessGuard(&blockHash) // release guardian
					continue
				}

				// add the total block processing time to the stats
				p.logger.Info("Processed block", slog.String("hash", blockHash.String()), slog.Int("txs", len(blockMsg.TransactionHashes)), slog.String("duration", time.Since(timeStart).String()))
				p.stopBlockProcessGuard(&blockHash) // release guardian
			}
		}
	}()
}

func (p *Processor) startBlockProcessGuard(ctx context.Context, hash *chainhash.Hash) {
	p.waitGroup.Add(1)

	execCtx, stopFn := context.WithCancel(ctx)
	p.processGuardsMap.Store(*hash, stopFn)

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

// do przerobienia - zwraca ostatni błąd co może oznaczać fałszywy nil
// powinno logować każdy błąd i zwracać hashe, które się nie udało wysłać
func (p *Processor) publishMinedTxs(txHashes []*chainhash.Hash) error {
	hashesBytes := make([][]byte, len(txHashes))
	for i, h := range txHashes {
		hashesBytes[i] = h[:]
	}

	minedTxs, err := p.store.GetMinedTransactions(p.ctx, hashesBytes, false)
	if err != nil {
		return fmt.Errorf("failed to get mined transactions: %v", err)
	}

	for _, minedTx := range minedTxs {
		txBlock := &blocktx_api.TransactionBlock{
			TransactionHash: minedTx.TxHash,
			BlockHash:       minedTx.BlockHash,
			BlockHeight:     minedTx.BlockHeight,
			MerklePath:      minedTx.MerklePath,
			BlockStatus:     minedTx.BlockStatus,
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

	blockHash := msg.Header.BlockHash()
	blockHeight := msg.Height

	p.logger.Info("processing incoming block", slog.String("hash", blockHash.String()))

	var chain chain
	var competing bool
	var err error

	// check if we've already processed that block
	existingBlock, _ := p.store.GetBlock(ctx, &blockHash)

	if existingBlock != nil && existingBlock.Processed {
		// if the block was already processed, check and update
		// possible orphan children of that block
		chain, competing, err = p.updateOrphans(ctx, existingBlock, false)
		if err != nil {
			p.logger.Error("unable to check and update possible orphaned child blocks", slog.String("hash", blockHash.String()), slog.String("err", err.Error()))
			return err
		}

		if len(chain) == 1 { // this means that no orphans were found
			p.logger.Warn("ignoring already existing block", slog.String("hash", blockHash.String()))
			return nil
		}
	} else {
		// if the block was not yet processed, proceed normally
		chain, competing, err = p.verifyAndInsertBlock(ctx, msg)
		if err != nil {
			return err
		}
	}

	chainTip, err := chain.getTip()
	if err != nil {
		p.logger.Error("unable to get chain tip", slog.String("hash", blockHash.String()), slog.String("err", err.Error()))
		return err
	}

	shouldPerformReorg := false
	if competing {
		shouldPerformReorg, err = p.hasGreatestChainwork(ctx, chainTip)
		if err != nil {
			p.logger.Error("unable to get the chain tip to verify chainwork", slog.String("hash", blockHash.String()), slog.Uint64("height", blockHeight), slog.String("err", err.Error()))
			return err
		}

		if shouldPerformReorg {
			p.logger.Info("chain reorg detected", slog.String("hash", blockHash.String()), slog.Uint64("height", blockHeight))

		}
	}

	txsToPublish := make([]store.TransactionBlock, 0)

	if shouldPerformReorg {
		txsToPublish, err = p.performReorg(ctx, chainTip)
		if err != nil {
			p.logger.Error("unable to perform reorg", slog.String("hash", blockHash.String()), slog.Uint64("height", blockHeight), slog.String("err", err.Error()))
			return err
		}
	} else if chainTip.Status == blocktx_api.Status_LONGEST {
		txsToPublish, err = p.store.GetRegisteredTxsByBlockHashes(ctx, chain.getHashes())
		if err != nil {
			p.logger.Error("unable to get registered transactions", slog.String("hash", blockHash.String()), slog.Uint64("height", blockHeight), slog.String("err", err.Error()))
			return err
		}
	}

	for _, tx := range txsToPublish {
		txBlock := &blocktx_api.TransactionBlock{
			BlockHash:       tx.BlockHash,
			BlockHeight:     tx.BlockHeight,
			TransactionHash: tx.TxHash,
			MerklePath:      tx.MerklePath,
			BlockStatus:     tx.BlockStatus,
		}

		p.logger.Info("publishing tx", slog.String("txHash", getHashStringNoErr(tx.TxHash)))

		err = p.mqClient.PublishMarshal(MinedTxsTopic, txBlock)
		if err != nil {
			p.logger.Error("failed to publish mined txs", slog.String("blockHash", getHashStringNoErr(tx.BlockHash)), slog.Uint64("height", tx.BlockHeight), slog.String("txHash", getHashStringNoErr(tx.TxHash)), slog.String("err", err.Error()))
		}
	}

	return nil
}

func (p *Processor) verifyAndInsertBlock(ctx context.Context, msg *p2p.BlockMessage) (chain, bool, error) {
	blockHash := msg.Header.BlockHash()
	previousBlockHash := msg.Header.PrevBlock

	// wrzuć  w create {
	prevBlock, err := p.getPrevBlock(ctx, &previousBlockHash)
	if err != nil {
		p.logger.Error("unable to get previous block from db", slog.String("hash", blockHash.String()), slog.Uint64("height", msg.Height), slog.String("prevHash", previousBlockHash.String()), slog.String("err", err.Error()))
		return nil, false, err
	}

	longestTipExists := true
	if prevBlock == nil {
		// This check is only in case there's a fresh, empty database
		// with no blocks, to mark the first block as the LONGEST chain
		longestTipExists, err = p.longestTipExists(ctx)
		if err != nil {
			p.logger.Error("unable to verify the longest tip existance in db", slog.String("hash", blockHash.String()), slog.Uint64("height", msg.Height), slog.String("err", err.Error()))
			return nil, false, err
		}
	}

	incomingBlock := createBlock(msg, prevBlock, longestTipExists)
	// }

	competing, err := p.competingChainsExist(ctx, incomingBlock)
	if err != nil {
		p.logger.Error("unable to check for competing chains", slog.String("hash", blockHash.String()), slog.Uint64("height", msg.Height), slog.String("err", err.Error()))
		return nil, false, err
	}

	if competing {
		p.logger.Info("Competing blocks found", slog.String("incoming block hash", blockHash.String()), slog.Uint64("height", incomingBlock.Height))
		incomingBlock.Status = blocktx_api.Status_STALE
	}

	p.logger.Info("Inserting block", slog.String("hash", blockHash.String()), slog.Uint64("height", incomingBlock.Height), slog.String("status", incomingBlock.Status.String()))

	err = p.insertBlockAndStoreTransactions(ctx, incomingBlock, msg.TransactionHashes, msg.Header.MerkleRoot)
	if err != nil {
		p.logger.Error("unable to insert block and store its transactions", slog.String("hash", blockHash.String()), slog.String("err", err.Error()))
		return nil, false, err
	}

	// if the block is ORPHANED, there's no need to process it any further
	if incomingBlock.Status == blocktx_api.Status_ORPHANED {
		return chain{incomingBlock}, false, nil
	}

	chain, competing, err := p.updateOrphans(ctx, incomingBlock, competing)
	if err != nil {
		p.logger.Error("unable to check and update possible orphaned child blocks", slog.String("hash", blockHash.String()), slog.String("err", err.Error()))
		return nil, false, err
	}

	return chain, competing, nil
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
		//rename getLongestBlokByHeigh
		competingBlock, err := p.store.GetBlockByHeight(ctx, block.Height)
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

func (p *Processor) hasGreatestChainwork(ctx context.Context, competingChainTip *blocktx_api.Block) (bool, error) {
	// mozna posortowac
	staleBlocks, err := p.store.GetStaleChainBackFromHash(ctx, competingChainTip.Hash)
	if err != nil {
		return false, err
	}

	lowestHeight := competingChainTip.Height
	if len(staleBlocks) > 0 {
		lowestHeight = getLowestHeight(staleBlocks)
	}

	longestBlocks, err := p.store.GetLongestChainFromHeight(ctx, lowestHeight)
	if err != nil {
		return false, err
	}

	sumStaleChainwork := big.NewInt(0)
	sumLongChainwork := big.NewInt(0)

	for _, b := range staleBlocks {
		chainwork := new(big.Int)
		chainwork.SetString(b.Chainwork, 10)
		sumStaleChainwork = sumStaleChainwork.Add(sumStaleChainwork, chainwork)
	}

	for _, b := range longestBlocks {
		chainwork := new(big.Int)
		chainwork.SetString(b.Chainwork, 10)
		sumLongChainwork = sumLongChainwork.Add(sumLongChainwork, chainwork)
	}

	return sumLongChainwork.Cmp(sumStaleChainwork) < 0, nil
}

func (p *Processor) insertBlockAndStoreTransactions(ctx context.Context, incomingBlock *blocktx_api.Block, txHashes []*chainhash.Hash, merkleRoot chainhash.Hash) error {
	blockID, err := p.store.UpsertBlock(ctx, incomingBlock)
	if err != nil {
		p.logger.Error("unable to insert block at given height", slog.String("hash", getHashStringNoErr(incomingBlock.Hash)), slog.Uint64("height", incomingBlock.Height), slog.String("err", err.Error()))
		return err
	}

	calculatedMerkleTree := p.buildMerkleTreeStoreChainHash(ctx, txHashes)

	if !merkleRoot.IsEqual(calculatedMerkleTree[len(calculatedMerkleTree)-1]) {
		p.logger.Error("merkle root mismatch", slog.String("hash", getHashStringNoErr(incomingBlock.Hash)))
		return err
	}

	if err = p.storeTransactions(ctx, blockID, incomingBlock, calculatedMerkleTree); err != nil {
		p.logger.Error("unable to store transactions from block", slog.String("hash", getHashStringNoErr(incomingBlock.Hash)), slog.String("err", err.Error()))
		return err
	}

	return nil
}

func (p *Processor) storeTransactions(ctx context.Context, blockID uint64, block *blocktx_api.Block, merkleTree []*chainhash.Hash) error {
	ctx, span := tracing.StartTracing(ctx, "markTransactionsAsMined", p.tracingEnabled, p.tracingAttributes...)
	defer tracing.EndTracing(span)
	txs := make([]store.TxWithMerklePath, 0, p.transactionStorageBatchSize)
	leaves := merkleTree[:(len(merkleTree)+1)/2]

	blockhash, err := chainhash.NewHash(block.Hash)
	if err != nil {
		return errors.Join(ErrFailedToParseBlockHash, fmt.Errorf("block height: %d", block.Height), err)
	}

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

		bump, err := bc.NewBUMPFromMerkleTreeAndIndex(block.Height, merkleTree, uint64(txIndex)) // NOSONAR
		if err != nil {
			ctx, iterateMerkleTree = tracing.StartTracing(ctx, "iterateMerkleTree", p.tracingEnabled, p.tracingAttributes...)
			// memory leak - niezamknięty iterateMerkleTree
			return errors.Join(ErrFailedToCreateBUMP, fmt.Errorf("tx hash %s, block height: %d", hash.String(), block.Height), err)
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
			err := p.store.UpsertBlockTransactions(ctx, blockID, txs)
			if err != nil {
				return errors.Join(ErrFailedToInsertBlockTransactions, err)
			}
			// free up memory
			txs = txs[:0]
		}

		if percentage, found := progress[txIndex+1]; found {
			if totalSize > 0 {
				p.logger.Info(fmt.Sprintf("%d txs out of %d stored", txIndex+1, totalSize), slog.Int("percentage", percentage), slog.String("hash", blockhash.String()), slog.Uint64("height", block.Height), slog.String("duration", time.Since(now).String()))
			}
		}
	}

	tracing.EndTracing(iterateMerkleTree)

	// update all remaining transactions
	err = p.store.UpsertBlockTransactions(ctx, blockID, txs)
	if err != nil {
		return errors.Join(ErrFailedToInsertBlockTransactions, fmt.Errorf("block height: %d", block.Height), err)
	}

	return nil
}

func (p *Processor) updateOrphans(ctx context.Context, incomingBlock *blocktx_api.Block, competing bool) (chain, bool, error) {
	chain := chain{incomingBlock}

	uow, err := p.store.StartUnitOfWork(ctx)
	if err != nil {
		return nil, false, err
	}
	defer func() {
		_ = uow.Rollback()
	}()

	// Very important step, this will lock blocks
	// table for writing but still allow reading.
	err = uow.WriteLockBlocksTable(ctx)
	if err != nil {
		return nil, false, err
	}

	orphanedBlocks, err := uow.GetOrphanedChainUpFromHash(ctx, incomingBlock.Hash)
	if err != nil {
		return nil, false, err
	}
	if len(orphanedBlocks) == 0 {
		return chain, competing, nil
	}

	// Orphany nie będą
	blockStatusUpdates := make([]store.BlockStatusUpdate, len(orphanedBlocks))
	for i := range orphanedBlocks {
		// We want to mark all orphaned blocks as STALE
		// in case there already exists a block at any
		// of their height with status LONGEST, which
		// would cause constraint validation (height, is_longest).
		//
		// If they are part of the LONGEST chain, the reorg
		// will happen and update their statuses accordingly.
		orphanedBlocks[i].Status = blocktx_api.Status_STALE

		blockStatusUpdates[i] = store.BlockStatusUpdate{
			Hash:   orphanedBlocks[i].Hash,
			Status: blocktx_api.Status_STALE,
		}
	}

	err = uow.UpdateBlocksStatuses(ctx, blockStatusUpdates)
	if err != nil {
		return nil, false, err
	}

	err = uow.Commit()
	if err != nil {
		return nil, false, err
	}

	p.logger.Info("orphans were found and updated", slog.Int("len", len(orphanedBlocks)))

	chain = append(chain, orphanedBlocks...)

	// if we found any orphans and marked them as STALE
	// we need to find out if they are part of the longest
	// or stale chain, so competing is returned as true
	return chain, true, nil
}

func (p *Processor) performReorg(ctx context.Context, staleChainTip *blocktx_api.Block) ([]store.TransactionBlock, error) {
	uow, err := p.store.StartUnitOfWork(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = uow.Rollback()
	}()

	// Very important step, this will lock blocks
	// table for writing but still allow reading.
	err = uow.WriteLockBlocksTable(ctx)
	if err != nil {
		return nil, err
	}

	staleBlocks, err := uow.GetStaleChainBackFromHash(ctx, staleChainTip.Hash)
	if err != nil {
		return nil, err
	}

	lowestHeight := staleChainTip.Height
	if len(staleBlocks) > 0 {
		lowestHeight = getLowestHeight(staleBlocks)
	}

	longestBlocks, err := uow.GetLongestChainFromHeight(ctx, lowestHeight)
	if err != nil {
		return nil, err
	}

	staleHashes := make([][]byte, len(staleBlocks))
	longestHashes := make([][]byte, len(longestBlocks))

	for i, b := range longestBlocks {
		longestHashes[i] = b.Hash
	}

	for i, b := range staleBlocks {
		staleHashes[i] = b.Hash
	}

	registeredTxs, err := uow.GetRegisteredTxsByBlockHashes(ctx, append(staleHashes, longestHashes...))
	if err != nil {
		return nil, err
	}

	// Order of inserting into blockStatusUpdates is important here, we need to do:
	// 1. LONGEST -> STALE
	// 2. STALE -> LONGEST
	// otherwise, a unique constraint on (height, is_longest) might be violated.

	// czemu nie jeden strzal?

	// 1. LONGEST -> STALE
	blockStatusUpdates := make([]store.BlockStatusUpdate, len(longestBlocks))
	for i, b := range longestBlocks {
		update := store.BlockStatusUpdate{Hash: b.Hash, Status: blocktx_api.Status_STALE}
		blockStatusUpdates[i] = update
	}

	err = uow.UpdateBlocksStatuses(ctx, blockStatusUpdates)
	if err != nil {
		return nil, err
	}

	// 2. STALE -> LONGEST
	blockStatusUpdates = make([]store.BlockStatusUpdate, len(staleBlocks))
	for _, b := range staleBlocks {
		update := store.BlockStatusUpdate{Hash: b.Hash, Status: blocktx_api.Status_LONGEST}
		blockStatusUpdates = append(blockStatusUpdates, update)
	}

	err = uow.UpdateBlocksStatuses(ctx, blockStatusUpdates)
	if err != nil {
		return nil, err
	}

	err = uow.Commit()
	if err != nil {
		return nil, err
	}

	p.logger.Info("reorg performed successfully")

	prevLongestTxs := make([]store.TransactionBlock, 0)
	prevStaleTxs := make([]store.TransactionBlock, 0)

	for _, tx := range registeredTxs {
		switch tx.BlockStatus {
		case blocktx_api.Status_LONGEST:
			prevLongestTxs = append(prevLongestTxs, tx)
		case blocktx_api.Status_STALE:
			prevStaleTxs = append(prevStaleTxs, tx)
		default:
			// nie mozemy tego ignorować
			// nie ma możliwości żeby tu były tego typu bloki
			// jesli są to mamy niespójne dane i jakaś część systemu jest jebnięta
			// powinniśmy albo ostro wywrócić system, albo zalogować ten błąd z informacją o potrzebie interwencji w bazę danych

			// do nothing - ignore ORPHANED and UNKNOWN blocks
		}
	}

	nowMinedTxs, nowStaleTxs := findMinedAndStaleTxs(prevStaleTxs, prevLongestTxs)

	for i := range nowMinedTxs {
		nowMinedTxs[i].BlockStatus = blocktx_api.Status_LONGEST
	}

	for i := range nowStaleTxs {
		nowStaleTxs[i].BlockStatus = blocktx_api.Status_STALE
	}

	txsToPublish := append(nowMinedTxs, nowStaleTxs...)

	return txsToPublish, nil
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
