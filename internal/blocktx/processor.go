package blocktx

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"sync"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/libsv/go-bc"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
	"go.opentelemetry.io/otel"
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

func WithMessageQueueClient(mqClient MessageQueueClient) func(handler *Processor) {
	return func(p *Processor) {
		p.mqClient = mqClient
	}
}

func WithTransactionBatchSize(size int) func(handler *Processor) {
	return func(p *Processor) {
		p.transactionStorageBatchSize = size
	}
}

func WithRetentionDays(dataRetentionDays int) func(handler *Processor) {
	return func(p *Processor) {
		p.dataRetentionDays = dataRetentionDays
	}
}

func WithFillGapsInterval(interval time.Duration) func(handler *Processor) {
	return func(handler *Processor) {
		handler.fillGapsInterval = interval
	}
}

func WithRegisterTxsInterval(d time.Duration) func(handler *Processor) {
	return func(p *Processor) {
		p.registerTxsInterval = d
	}
}

func WithRegisterRequestTxsInterval(d time.Duration) func(handler *Processor) {
	return func(p *Processor) {
		p.registerRequestTxsInterval = d
	}
}

func WithRegisterTxsChan(registerTxsChan chan []byte) func(handler *Processor) {
	return func(handler *Processor) {
		handler.registerTxsChan = registerTxsChan
	}
}

func WithRequestTxChan(requestTxChannel chan []byte) func(handler *Processor) {
	return func(handler *Processor) {
		handler.requestTxChannel = requestTxChannel
	}
}

func WithRegisterTxsBatchSize(size int) func(handler *Processor) {
	return func(handler *Processor) {
		handler.registerTxsBatchSize = size
	}
}

func WithRegisterRequestTxsBatchSize(size int) func(handler *Processor) {
	return func(handler *Processor) {
		handler.registerRequestTxsBatchSize = size
	}
}

func WithTracer() func(handler *Processor) {
	return func(_ *Processor) {
		tracer = otel.GetTracerProvider().Tracer("")
	}
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

	ph := &Processor{
		store:                       storeI,
		logger:                      logger,
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
		opt(ph)
	}

	ctx, cancelAll := context.WithCancel(context.Background())
	ph.cancelAll = cancelAll
	ph.ctx = ctx

	return ph, nil
}

func (ph *Processor) Start() error {
	err := ph.mqClient.Subscribe(RegisterTxTopic, func(msg []byte) error {
		ph.registerTxsChan <- msg
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to %s topic: %w", RegisterTxTopic, err)
	}

	err = ph.mqClient.Subscribe(RequestTxTopic, func(msg []byte) error {
		ph.requestTxChannel <- msg
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to %s topic: %w", RequestTxTopic, err)
	}

	ph.StartBlockRequesting()
	ph.StartBlockProcessing()
	ph.StartProcessRegisterTxs()
	ph.StartProcessRequestTxs()

	return nil
}

func (ph *Processor) StartBlockRequesting() {
	ph.waitGroup.Add(1)

	go func() {
		defer ph.waitGroup.Done()
		for {
			select {
			case <-ph.ctx.Done():
				return
			case req := <-ph.blockRequestCh:
				hash := req.Hash
				peer := req.Peer

				bhs, err := ph.store.GetBlockHashesProcessingInProgress(ph.ctx, ph.hostname)
				if err != nil {
					ph.logger.Error("failed to get block hashes where processing in progress", slog.String("err", err.Error()))
				}

				if len(bhs) >= maxBlocksInProgress {
					ph.logger.Debug("max blocks being processed reached", slog.String("hash", hash.String()), slog.Int("max", maxBlocksInProgress), slog.Int("number", len(bhs)))
					continue
				}

				processedBy, err := ph.store.SetBlockProcessing(ph.ctx, hash, ph.hostname)
				if err != nil {
					// block is already being processed by another blocktx instance
					if errors.Is(err, store.ErrBlockProcessingDuplicateKey) {
						ph.logger.Debug("block processing already in progress", slog.String("hash", hash.String()), slog.String("processed_by", processedBy))
						continue
					}

					ph.logger.Error("failed to set block processing", slog.String("hash", hash.String()))
					continue
				}

				msg := wire.NewMsgGetData()
				if err = msg.AddInvVect(wire.NewInvVect(wire.InvTypeBlock, hash)); err != nil {
					ph.logger.Error("Failed to create InvVect for block request", slog.String("hash", hash.String()), slog.String("err", err.Error()))
					continue
				}

				if err = peer.WriteMsg(msg); err != nil {
					ph.logger.Error("Failed to write block request message to peer", slog.String("hash", hash.String()), slog.String("err", err.Error()))
					continue
				}

				ph.logger.Info("Block request message sent to peer", slog.String("hash", hash.String()), slog.String("peer", peer.String()))
			}
		}
	}()
}

func (ph *Processor) StartBlockProcessing() {
	ph.waitGroup.Add(1)

	go func() {
		defer ph.waitGroup.Done()

		for {
			select {
			case <-ph.ctx.Done():
				return
			case blockMsg := <-ph.blockProcessCh:
				ph.processBlock(blockMsg)
			}
		}
	}()
}

func (ph *Processor) StartFillGaps(peers []p2p.PeerI) {
	ph.waitGroup.Add(1)

	ticker := time.NewTicker(ph.fillGapsInterval)
	go func() {
		defer ph.waitGroup.Done()
		peerIndex := 0
		for {
			select {
			case <-ph.ctx.Done():
				return
			case <-ticker.C:
				if peerIndex >= len(peers) {
					peerIndex = 0
				}

				err := ph.fillGaps(peers[peerIndex])
				if err != nil {
					ph.logger.Error("failed to fill gaps", slog.String("error", err.Error()))
				}

				peerIndex++
				ticker.Reset(ph.fillGapsInterval)
			}
		}
	}()
}

func (ph *Processor) StartProcessRegisterTxs() {
	ph.waitGroup.Add(1)
	txHashes := make([]*blocktx_api.TransactionAndSource, 0, ph.registerTxsBatchSize)

	ticker := time.NewTicker(ph.registerTxsInterval)
	go func() {
		defer ph.waitGroup.Done()
		for {
			select {
			case <-ph.ctx.Done():
				return
			case txHash := <-ph.registerTxsChan:
				txHashes = append(txHashes, &blocktx_api.TransactionAndSource{
					Hash: txHash,
				})

				if len(txHashes) < ph.registerTxsBatchSize {
					continue
				}

				ph.registerTransactions(txHashes[:])
				txHashes = make([]*blocktx_api.TransactionAndSource, 0, ph.registerTxsBatchSize)
				ticker.Reset(ph.registerTxsInterval)

			case <-ticker.C:
				if len(txHashes) == 0 {
					continue
				}

				ph.registerTransactions(txHashes[:])
				txHashes = make([]*blocktx_api.TransactionAndSource, 0, ph.registerTxsBatchSize)
				ticker.Reset(ph.registerTxsInterval)
			}
		}
	}()
}

func (ph *Processor) StartProcessRequestTxs() {
	ph.waitGroup.Add(1)

	txHashes := make([]*chainhash.Hash, 0, ph.registerRequestTxsBatchSize)

	ticker := time.NewTicker(ph.registerRequestTxsInterval)

	go func() {
		defer ph.waitGroup.Done()

		for {
			select {
			case <-ph.ctx.Done():
				return
			case txHash := <-ph.requestTxChannel:
				tx, err := chainhash.NewHash(txHash)
				if err != nil {
					ph.logger.Error("Failed to create hash from byte array", slog.String("err", err.Error()))
					continue
				}

				txHashes = append(txHashes, tx)

				if len(txHashes) < ph.registerRequestTxsBatchSize || len(txHashes) == 0 {
					continue
				}

				err = ph.publishMinedTxs(txHashes)
				if err != nil {
					ph.logger.Error("failed to publish mined txs", slog.String("err", err.Error()))
					continue
				}

				txHashes = make([]*chainhash.Hash, 0, ph.registerRequestTxsBatchSize)
				ticker.Reset(ph.registerRequestTxsInterval)

			case <-ticker.C:
				if len(txHashes) == 0 {
					continue
				}

				err := ph.publishMinedTxs(txHashes)
				if err != nil {
					ph.logger.Error("failed to publish mined txs", slog.String("err", err.Error()))
					continue
				}

				txHashes = make([]*chainhash.Hash, 0, ph.registerRequestTxsBatchSize)
				ticker.Reset(ph.registerRequestTxsInterval)
			}
		}
	}()
}

func (ph *Processor) publishMinedTxs(txHashes []*chainhash.Hash) error {
	minedTxs, err := ph.store.GetMinedTransactions(ph.ctx, txHashes)
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
		err = ph.mqClient.PublishMarshal(MinedTxsTopic, txBlock)
	}

	if err != nil {
		return fmt.Errorf("failed to publish mined transactions: %v", err)
	}

	return nil
}

func (ph *Processor) registerTransactions(txHashes []*blocktx_api.TransactionAndSource) {
	updatedTxs, err := ph.store.RegisterTransactions(ph.ctx, txHashes)
	if err != nil {
		ph.logger.Error("failed to register transactions", slog.String("err", err.Error()))
	}

	if len(updatedTxs) > 0 {
		err = ph.publishMinedTxs(updatedTxs)
		if err != nil {
			ph.logger.Error("failed to publish mined txs", slog.String("err", err.Error()))
		}
	}
}

const (
	hoursPerDay   = 24
	blocksPerHour = 6
)

func (ph *Processor) fillGaps(peer p2p.PeerI) error {
	heightRange := ph.dataRetentionDays * hoursPerDay * blocksPerHour

	blockHeightGaps, err := ph.store.GetBlockGaps(ph.ctx, heightRange)
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

		ph.logger.Info("Requesting missing block", slog.String("hash", gaps.Hash.String()), slog.Int64("height", int64(gaps.Height)), slog.String("peer", peer.String()))

		pair := BlockRequest{
			Hash: gaps.Hash,
			Peer: peer,
		}
		ph.blockRequestCh <- pair
	}

	return nil
}

func (ph *Processor) insertBlock(ctx context.Context, blockHash *chainhash.Hash, merkleRoot *chainhash.Hash, previousBlockHash *chainhash.Hash, height uint64) (uint64, error) {
	ph.logger.Info("Inserting block", slog.String("hash", blockHash.String()), slog.Int64("height", int64(height)))

	block := &blocktx_api.Block{
		Hash:         blockHash[:],
		MerkleRoot:   merkleRoot[:],
		PreviousHash: previousBlockHash[:],
		Height:       height,
	}

	return ph.store.InsertBlock(ctx, block)
}

func buildMerkleTreeStoreChainHash(ctx context.Context, txids []*chainhash.Hash) []*chainhash.Hash {
	if tracer != nil {
		var span trace.Span
		_, span = tracer.Start(ctx, "buildMerkleTreeStoreChainHash")
		defer span.End()
	}

	return bc.BuildMerkleTreeStoreChainHash(txids)
}

func (ph *Processor) processBlock(msg *p2p.BlockMessage) {
	ctx := ph.ctx

	if tracer != nil {
		var span trace.Span
		ctx, span = tracer.Start(ctx, "HandleBlock")
		defer span.End()
	}

	timeStart := time.Now()

	blockHash := msg.Header.BlockHash()

	previousBlockHash := msg.Header.PrevBlock

	merkleRoot := msg.Header.MerkleRoot

	blockId, err := ph.insertBlock(ctx, &blockHash, &merkleRoot, &previousBlockHash, msg.Height)
	if err != nil {
		_, errDel := ph.store.DelBlockProcessing(ctx, &blockHash, ph.hostname)
		if errDel != nil {
			ph.logger.Error("failed to delete block processing - after inserting block failed", slog.String("hash", blockHash.String()), slog.String("err", errDel.Error()))
		}
		ph.logger.Error("unable to insert block at given height", slog.String("hash", blockHash.String()), slog.Uint64("height", msg.Height), slog.String("err", err.Error()))
		return
	}

	calculatedMerkleTree := buildMerkleTreeStoreChainHash(ctx, msg.TransactionHashes)

	if !merkleRoot.IsEqual(calculatedMerkleTree[len(calculatedMerkleTree)-1]) {
		_, errDel := ph.store.DelBlockProcessing(ctx, &blockHash, ph.hostname)
		if errDel != nil {
			ph.logger.Error("failed to delete block processing - after merkle root mismatch", slog.String("hash", blockHash.String()), slog.String("err", errDel.Error()))
		}
		ph.logger.Error("merkle root mismatch", slog.String("hash", blockHash.String()))
		return
	}

	if err = ph.markTransactionsAsMined(ctx, blockId, calculatedMerkleTree, msg.Height, &blockHash); err != nil {
		_, errDel := ph.store.DelBlockProcessing(ctx, &blockHash, ph.hostname)
		if errDel != nil {
			ph.logger.Error("failed to delete block processing - after marking transactions as mined failed", slog.String("hash", blockHash.String()), slog.String("err", errDel.Error()))
		}
		ph.logger.Error("unable to mark block as mined", slog.String("hash", blockHash.String()), slog.String("err", err.Error()))
		return
	}

	block := &p2p.Block{
		Hash:         &blockHash,
		MerkleRoot:   &merkleRoot,
		PreviousHash: &previousBlockHash,
		Height:       msg.Height,
		Size:         msg.Size,
		TxCount:      uint64(len(msg.TransactionHashes)),
	}

	if err = ph.markBlockAsProcessed(ctx, block); err != nil {
		_, errDel := ph.store.DelBlockProcessing(ctx, &blockHash, ph.hostname)
		if errDel != nil {
			ph.logger.Error("failed to delete block processing - after marking block as processed failed", slog.String("hash", blockHash.String()), slog.String("err", errDel.Error()))
		}
		ph.logger.Error("unable to mark block as processed", slog.String("hash", blockHash.String()), slog.String("err", err.Error()))
		return
	}

	// add the total block processing time to the stats
	ph.logger.Info("Processed block", slog.String("hash", blockHash.String()), slog.Int("txs", len(msg.TransactionHashes)), slog.String("duration", time.Since(timeStart).String()))
}

func (ph *Processor) markTransactionsAsMined(ctx context.Context, blockId uint64, merkleTree []*chainhash.Hash, blockHeight uint64, blockhash *chainhash.Hash) error {
	if tracer != nil {
		var span trace.Span
		ctx, span = tracer.Start(ctx, "markTransactionsAsMined")
		defer span.End()
	}
	txs := make([]*blocktx_api.TransactionAndSource, 0, ph.transactionStorageBatchSize)
	merklePaths := make([]string, 0, ph.transactionStorageBatchSize)
	leaves := merkleTree[:(len(merkleTree)+1)/2]

	totalSize := 0
	for txIndex, hash := range leaves {
		if hash == nil {
			totalSize = txIndex
			break
		}
	}

	step := int(math.Ceil(float64(totalSize) / 5))
	progressIndices := map[int]int{step: 20, step * 2: 40, step * 3: 60, step * 4: 80, step * 5: 100}

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

		if percentage, found := progressIndices[txIndex]; found {
			if totalSize > 0 {
				ph.logger.Info(fmt.Sprintf("%d txs out of %d marked as mined", txIndex, totalSize), slog.Int("percentage", percentage), slog.String("hash", blockhash.String()), slog.Int64("height", int64(blockHeight)), slog.String("duration", time.Since(now).String()))
			}
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
		if (txIndex+1)%ph.transactionStorageBatchSize == 0 {
			updateResp, err := ph.store.UpsertBlockTransactions(ctx, blockId, txs, merklePaths)
			if err != nil {
				return fmt.Errorf("failed to insert block transactions at block height %d: %v", blockHeight, err)
			}
			// free up memory
			txs = make([]*blocktx_api.TransactionAndSource, 0, ph.transactionStorageBatchSize)
			merklePaths = make([]string, 0, ph.transactionStorageBatchSize)

			for _, updResp := range updateResp {
				txBlock := &blocktx_api.TransactionBlock{
					TransactionHash: updResp.TxHash[:],
					BlockHash:       blockhash[:],
					BlockHeight:     blockHeight,
					MerklePath:      updResp.MerklePath,
				}
				err = ph.mqClient.PublishMarshal(MinedTxsTopic, txBlock)
				if err != nil {
					ph.logger.Error("failed to publish mined txs", slog.String("hash", blockhash.String()), slog.Int64("height", int64(blockHeight)), slog.String("err", err.Error()))
				}
			}
		}
	}

	if iterateMerkleTree != nil {
		iterateMerkleTree.End()
	}

	// update all remaining transactions
	updateResp, err := ph.store.UpsertBlockTransactions(ctx, blockId, txs, merklePaths)
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
		err = ph.mqClient.PublishMarshal(MinedTxsTopic, txBlock)
		if err != nil {
			ph.logger.Error("failed to publish mined txs", slog.String("hash", blockhash.String()), slog.Int64("height", int64(blockHeight)), slog.String("err", err.Error()))
		}
	}

	return nil
}

func (ph *Processor) markBlockAsProcessed(ctx context.Context, block *p2p.Block) error {
	err := ph.store.MarkBlockAsDone(ctx, block.Hash, block.Size, block.TxCount)
	if err != nil {
		return err
	}

	return nil
}

// exported for testing purposes
func ExtractHeightFromCoinbaseTx(tx *bt.Tx) uint64 {
	// Coinbase tx has a special format, the height is encoded in the first 4 bytes of the scriptSig
	// https://en.bitcoin.it/wiki/Protocol_documentation#tx
	// Get the length
	script := *(tx.Inputs[0].UnlockingScript)
	length := int(script[0])

	if len(script) < length+1 {
		return 0
	}

	b := make([]byte, 8)

	for i := 0; i < length; i++ {
		b[i] = script[i+1]
	}

	return binary.LittleEndian.Uint64(b)
}

func (ph *Processor) Shutdown() {
	if ph.cancelAll != nil {
		ph.cancelAll()
	}
	ph.waitGroup.Wait()
}

// for testing purposes
func (ph *Processor) GetWorkerCh() chan BlockRequest {
	return ph.blockRequestCh
}
