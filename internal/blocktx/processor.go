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

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/libsv/go-bc"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
	"go.opentelemetry.io/otel/trace"
)

var tracer trace.Tracer

const (
	transactionStoringBatchsizeDefault = 8192 // power of 2 for easier memory allocation
	maxRequestBlocks                   = 1
	maxBlocksInProgress                = 1
	fillGapsInterval                   = 15 * time.Minute
	registerTxsIntervalDefault         = time.Second * 10
	registerRequestTxsIntervalDefault  = time.Second * 5
	registerTxsBatchSizeDefault        = 100
	registerRequestTxBatchSizeDefault  = 100
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
	fillGapsInterval            time.Duration

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
		fillGapsInterval:            fillGapsInterval,
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
		return fmt.Errorf("failed to subscribe to %s topic: %w", RegisterTxTopic, err)
	}

	err = p.mqClient.Subscribe(RequestTxTopic, func(msg []byte) error {
		p.requestTxChannel <- msg
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to %s topic: %w", RequestTxTopic, err)
	}

	p.StartBlockRequesting()
	p.StartBlockProcessing()
	p.StartProcessRegisterTxs()
	p.StartProcessRequestTxs()

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

				bhs, err := p.store.GetBlockHashesProcessingInProgress(p.ctx, p.hostname)
				if err != nil {
					p.logger.Error("failed to get block hashes where processing in progress", slog.String("err", err.Error()))
				}

				if len(bhs) >= maxBlocksInProgress {
					p.logger.Debug("max blocks being processed reached", slog.String("hash", hash.String()), slog.Int("max", maxBlocksInProgress), slog.Int("number", len(bhs)))
					continue
				}

				processedBy, err := p.store.SetBlockProcessing(p.ctx, hash, p.hostname)
				if err != nil {
					// block is already being processed by another blocktx instance
					if errors.Is(err, store.ErrBlockProcessingDuplicateKey) {
						p.logger.Debug("block processing already in progress", slog.String("hash", hash.String()), slog.String("processed_by", processedBy))
						continue
					}

					p.logger.Error("failed to set block processing", slog.String("hash", hash.String()))
					continue
				}

				msg := wire.NewMsgGetData()
				if err = msg.AddInvVect(wire.NewInvVect(wire.InvTypeBlock, hash)); err != nil {
					p.logger.Error("Failed to create InvVect for block request", slog.String("hash", hash.String()), slog.String("err", err.Error()))
					continue
				}

				if err = peer.WriteMsg(msg); err != nil {
					p.logger.Error("Failed to write block request message to peer", slog.String("hash", hash.String()), slog.String("err", err.Error()))
					continue
				}

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
				err := p.processBlock(blockMsg)
				if err != nil {
					blockhash := blockMsg.Header.BlockHash()
					_, errDel := p.store.DelBlockProcessing(p.ctx, &blockhash, p.hostname)
					if errDel != nil {
						p.logger.Error("failed to delete block processing", slog.String("hash", blockhash.String()), slog.String("err", errDel.Error()))
					}
				}
			}
		}
	}()
}

func (p *Processor) StartFillGaps(peers []p2p.PeerI) {
	p.waitGroup.Add(1)

	ticker := time.NewTicker(p.fillGapsInterval)
	go func() {
		defer p.waitGroup.Done()
		peerIndex := 0
		for {
			select {
			case <-p.ctx.Done():
				return
			case <-ticker.C:
				if peerIndex >= len(peers) {
					peerIndex = 0
				}

				err := p.fillGaps(peers[peerIndex])
				if err != nil {
					p.logger.Error("failed to fill gaps", slog.String("error", err.Error()))
				}

				peerIndex++
				ticker.Reset(p.fillGapsInterval)
			}
		}
	}()
}

func (p *Processor) StartProcessRegisterTxs() {
	p.waitGroup.Add(1)
	txHashes := make([]*blocktx_api.TransactionAndSource, 0, p.registerTxsBatchSize)

	ticker := time.NewTicker(p.registerTxsInterval)
	go func() {
		defer p.waitGroup.Done()
		for {
			select {
			case <-p.ctx.Done():
				return
			case txHash := <-p.registerTxsChan:
				txHashes = append(txHashes, &blocktx_api.TransactionAndSource{
					Hash: txHash,
				})

				if len(txHashes) < p.registerTxsBatchSize {
					continue
				}

				p.registerTransactions(txHashes[:])
				txHashes = make([]*blocktx_api.TransactionAndSource, 0, p.registerTxsBatchSize)
				ticker.Reset(p.registerTxsInterval)

			case <-ticker.C:
				if len(txHashes) == 0 {
					continue
				}

				p.registerTransactions(txHashes[:])
				txHashes = make([]*blocktx_api.TransactionAndSource, 0, p.registerTxsBatchSize)
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

func (p *Processor) registerTransactions(txHashes []*blocktx_api.TransactionAndSource) {
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

const (
	hoursPerDay   = 24
	blocksPerHour = 6
)

func (p *Processor) fillGaps(peer p2p.PeerI) error {
	heightRange := p.dataRetentionDays * hoursPerDay * blocksPerHour

	blockHeightGaps, err := p.store.GetBlockGaps(p.ctx, heightRange)
	if err != nil {
		return err
	}

	if len(blockHeightGaps) == 0 {
		return nil
	}

	for i, gaps := range blockHeightGaps {
		if i+1 > maxRequestBlocks {
			break
		}

		p.logger.Info("Requesting missing block", slog.String("hash", gaps.Hash.String()), slog.Int64("height", int64(gaps.Height)), slog.String("peer", peer.String()))

		pair := BlockRequest{
			Hash: gaps.Hash,
			Peer: peer,
		}
		p.blockRequestCh <- pair
	}

	return nil
}

func buildMerkleTreeStoreChainHash(ctx context.Context, txids []*chainhash.Hash) []*chainhash.Hash {
	if tracer != nil {
		var span trace.Span
		_, span = tracer.Start(ctx, "buildMerkleTreeStoreChainHash")
		defer span.End()
	}

	return bc.BuildMerkleTreeStoreChainHash(txids)
}

func (p *Processor) processBlock(msg *p2p.BlockMessage) error {
	ctx := p.ctx

	if tracer != nil {
		var span trace.Span
		ctx, span = tracer.Start(ctx, "HandleBlock")
		defer span.End()
	}

	timeStart := time.Now()

	blockHash := msg.Header.BlockHash()
	previousBlockHash := msg.Header.PrevBlock
	merkleRoot := msg.Header.MerkleRoot

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

		incomingBlock.Status = blocktx_api.Status_STALE

		if hasGreatestChainwork {
			// TODO: perform reorg - next ticket
			incomingBlock.Status = blocktx_api.Status_LONGEST
			err := p.performReorg()
			if err != nil {
				// TODO: error log
				return err
			}
		}
	}

	p.logger.Info("Inserting block", slog.String("hash", blockHash.String()), slog.Uint64("height", incomingBlock.Height), slog.String("status", incomingBlock.Status.String()))

	blockId, err := p.store.InsertBlock(ctx, incomingBlock)
	if err != nil {
		p.logger.Error("unable to insert block at given height", slog.String("hash", blockHash.String()), slog.Uint64("height", msg.Height), slog.String("err", err.Error()))
		return err
	}

	calculatedMerkleTree := buildMerkleTreeStoreChainHash(ctx, msg.TransactionHashes)

	if !merkleRoot.IsEqual(calculatedMerkleTree[len(calculatedMerkleTree)-1]) {
		p.logger.Error("merkle root mismatch", slog.String("hash", blockHash.String()))
		return err
	}

	if err = p.markTransactionsAsMined(ctx, blockId, calculatedMerkleTree, msg.Height, &blockHash); err != nil {
		p.logger.Error("unable to mark block as mined", slog.String("hash", blockHash.String()), slog.String("err", err.Error()))
		return err
	}

	block := &p2p.Block{
		Hash:         &blockHash,
		MerkleRoot:   &merkleRoot,
		PreviousHash: &previousBlockHash,
		Height:       msg.Height,
		Size:         msg.Size,
		TxCount:      uint64(len(msg.TransactionHashes)),
	}

	if err = p.markBlockAsProcessed(ctx, block); err != nil {
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

func (p *Processor) performReorg() error {
	// get all blocks until the common ancestor of competing chains
	// 		get entire STALE chain by performing a recursive query
	// 		get lowest height of that chain
	// 		get all headers from the longest chain from that height
	// replace statuses of these blocks
	return nil
}

func (p *Processor) markTransactionsAsMined(ctx context.Context, blockId uint64, merkleTree []*chainhash.Hash, blockHeight uint64, blockhash *chainhash.Hash) error {
	if tracer != nil {
		var span trace.Span
		ctx, span = tracer.Start(ctx, "markTransactionsAsMined")
		defer span.End()
	}
	txs := make([]*blocktx_api.TransactionAndSource, 0, p.transactionStorageBatchSize)
	merklePaths := make([]string, 0, p.transactionStorageBatchSize)
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
	if tracer != nil {
		ctx, iterateMerkleTree = tracer.Start(ctx, "iterateMerkleTree")
	}

	for txIndex, hash := range leaves {
		// Everything to the right of the first nil will also be nil, as this is just padding upto the next PoT.
		if hash == nil {
			break
		}

		// Otherwise they're txids, which should have merkle paths calculated.
		txs = append(txs, &blocktx_api.TransactionAndSource{
			Hash: hash[:],
		})

		bump, err := bc.NewBUMPFromMerkleTreeAndIndex(blockHeight, merkleTree, uint64(txIndex))
		if err != nil {
			return fmt.Errorf("failed to create new bump for tx hash %s from merkle tree and index at block height %d: %v", hash.String(), blockHeight, err)
		}

		bumpHex, err := bump.String()
		if err != nil {
			return fmt.Errorf("failed to get string from bump for tx hash %s at block height %d: %v", hash.String(), blockHeight, err)
		}

		merklePaths = append(merklePaths, bumpHex)
		if (txIndex+1)%p.transactionStorageBatchSize == 0 {
			updateResp, err := p.store.UpsertBlockTransactions(ctx, blockId, txs, merklePaths)
			if err != nil {
				return fmt.Errorf("failed to insert block transactions at block height %d: %v", blockHeight, err)
			}
			// free up memory
			txs = make([]*blocktx_api.TransactionAndSource, 0, p.transactionStorageBatchSize)
			merklePaths = make([]string, 0, p.transactionStorageBatchSize)

			for _, updResp := range updateResp {
				txBlock := &blocktx_api.TransactionBlock{
					TransactionHash: updResp.TxHash[:],
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

	if iterateMerkleTree != nil {
		iterateMerkleTree.End()
	}

	// update all remaining transactions
	updateResp, err := p.store.UpsertBlockTransactions(ctx, blockId, txs, merklePaths)
	if err != nil {
		return fmt.Errorf("failed to insert block transactions at block height %d: %v", blockHeight, err)
	}

	for _, updResp := range updateResp {
		txBlock := &blocktx_api.TransactionBlock{
			TransactionHash: updResp.TxHash[:],
			BlockHash:       blockhash[:],
			BlockHeight:     blockHeight,
			MerklePath:      updResp.MerklePath,
		}
		err = p.mqClient.PublishMarshal(MinedTxsTopic, txBlock)
		if err != nil {
			p.logger.Error("failed to publish mined txs", slog.String("hash", blockhash.String()), slog.Int64("height", int64(blockHeight)), slog.String("err", err.Error()))
		}
	}

	return nil
}

func (p *Processor) markBlockAsProcessed(ctx context.Context, block *p2p.Block) error {
	err := p.store.MarkBlockAsDone(ctx, block.Hash, block.Size, block.TxCount)
	if err != nil {
		return err
	}

	return nil
}

func (p *Processor) Shutdown() {
	p.cancelAll()
	p.waitGroup.Wait()
}

// for testing purposes only
func (p *Processor) GetBlockRequestCh() chan BlockRequest {
	return p.blockRequestCh
}
