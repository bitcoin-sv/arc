package blocktx

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"runtime"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/bitcoin-sv/arc/internal/metrics"
	"github.com/bitcoin-sv/arc/pkg/blocktx/blocktx_api"
	"github.com/libsv/go-bc"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
	"github.com/ordishs/go-utils/safemap"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

var tracer trace.Tracer

const (
	transactionStoringBatchsizeDefault = 8192 // power of 2 for easier memory allocation
	maxRequestBlocks                   = 1
	fillGapsInterval                   = 15 * time.Minute
	registerTxsIntervalDefault         = time.Second * 10
	registerRequestTxsIntervalDefault  = time.Second * 5
	registerTxsBatchSizeDefault        = 100
	registerRequestTxBatchSizeDefault  = 100
	maxBlocksInProgress                = 1
)

func init() {
	// override the default wire block handler with our own that streams and stores only the transaction ids
	wire.SetExternalHandler(wire.CmdBlock, func(reader io.Reader, length uint64, bytesRead int) (int, wire.Message, []byte, error) {
		blockMessage := &p2p.BlockMessage{
			Header: &wire.BlockHeader{},
		}

		err := blockMessage.Header.Deserialize(reader)
		if err != nil {
			return bytesRead, nil, nil, err
		}
		bytesRead += 80 // the bitcoin header is always 80 bytes

		var read int64
		var txCount bt.VarInt
		read, err = txCount.ReadFrom(reader)
		if err != nil {
			return bytesRead, nil, nil, err
		}
		bytesRead += int(read)

		blockMessage.TransactionHashes = make([]*chainhash.Hash, txCount)

		var tx *bt.Tx
		var hash *chainhash.Hash
		var txBytes []byte
		for i := 0; i < int(txCount); i++ {
			tx = bt.NewTx()
			read, err = tx.ReadFrom(reader)
			if err != nil {
				return bytesRead, nil, nil, err
			}
			bytesRead += int(read)
			txBytes = tx.TxIDBytes() // this returns the bytes in BigEndian
			hash, err = chainhash.NewHash(bt.ReverseBytes(txBytes))
			if err != nil {
				return 0, nil, nil, err
			}

			blockMessage.TransactionHashes[i] = hash

			if i == 0 {
				blockMessage.Height = extractHeightFromCoinbaseTx(tx)
			}
		}

		blockMessage.Size = uint64(bytesRead)

		return bytesRead, blockMessage, nil, nil
	})
}

type hashPeer struct {
	Hash *chainhash.Hash
	Peer p2p.PeerI
}

type PeerHandler struct {
	hostname                    string
	workerCh                    chan hashPeer
	store                       store.BlocktxStore
	logger                      *slog.Logger
	stats                       *safemap.Safemap[string, *metrics.PeerHandlerStats]
	transactionStorageBatchSize int
	peerHandlerCollector        *metrics.PeerHandlerCollector
	dataRetentionDays           int
	mqClient                    MessageQueueClient
	txChannel                   chan []byte
	requestTxChannel            chan []byte
	registerTxsInterval         time.Duration
	registerRequestTxsInterval  time.Duration
	registerTxsBatchSize        int
	registerRequestTxsBatchSize int

	fillGapsTicker                     *time.Ticker
	quitFillBlockGap                   chan struct{}
	quitFillBlockGapComplete           chan struct{}
	quitPeerWorker                     chan struct{}
	quitPeerWorkerComplete             chan struct{}
	quitListenTxChannel                chan struct{}
	quitListenTxChannelComplete        chan struct{}
	cancelListenRequestTxChannel       context.CancelFunc
	quitListenRequestTxChannelComplete chan struct{}
}

func WithMessageQueueClient(mqClient MessageQueueClient) func(handler *PeerHandler) {
	return func(p *PeerHandler) {
		p.mqClient = mqClient
	}
}

func WithTransactionBatchSize(size int) func(handler *PeerHandler) {
	return func(p *PeerHandler) {
		p.transactionStorageBatchSize = size
	}
}

func WithRetentionDays(dataRetentionDays int) func(handler *PeerHandler) {
	return func(p *PeerHandler) {
		p.dataRetentionDays = dataRetentionDays
	}
}

func WithFillGapsInterval(interval time.Duration) func(handler *PeerHandler) {
	return func(handler *PeerHandler) {
		handler.fillGapsTicker = time.NewTicker(interval)
	}
}

func WithRegisterTxsInterval(d time.Duration) func(handler *PeerHandler) {
	return func(p *PeerHandler) {
		p.registerTxsInterval = d
	}
}

func WithRegisterRequestTxsInterval(d time.Duration) func(handler *PeerHandler) {
	return func(p *PeerHandler) {
		p.registerRequestTxsInterval = d
	}
}

func WithTxChan(txChannel chan []byte) func(handler *PeerHandler) {
	return func(handler *PeerHandler) {
		handler.txChannel = txChannel
	}
}

func WithRequestTxChan(requestTxChannel chan []byte) func(handler *PeerHandler) {
	return func(handler *PeerHandler) {
		handler.requestTxChannel = requestTxChannel
	}
}

func WithRegisterTxsBatchSize(size int) func(handler *PeerHandler) {
	return func(handler *PeerHandler) {
		handler.registerTxsBatchSize = size
	}
}

func WithRegisterRequestTxsBatchSize(size int) func(handler *PeerHandler) {
	return func(handler *PeerHandler) {
		handler.registerRequestTxsBatchSize = size
	}
}

func WithTracer() func(handler *PeerHandler) {
	return func(_ *PeerHandler) {
		tracer = otel.GetTracerProvider().Tracer("")
	}
}

func NewPeerHandler(logger *slog.Logger, storeI store.BlocktxStore, opts ...func(*PeerHandler)) (*PeerHandler, error) {

	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	ph := &PeerHandler{
		store:                       storeI,
		logger:                      logger,
		workerCh:                    make(chan hashPeer, 100),
		stats:                       safemap.New[string, *metrics.PeerHandlerStats](),
		transactionStorageBatchSize: transactionStoringBatchsizeDefault,
		registerTxsInterval:         registerTxsIntervalDefault,
		registerRequestTxsInterval:  registerRequestTxsIntervalDefault,
		registerTxsBatchSize:        registerTxsBatchSizeDefault,
		registerRequestTxsBatchSize: registerRequestTxBatchSizeDefault,
		hostname:                    hostname,

		fillGapsTicker: time.NewTicker(fillGapsInterval),
	}

	for _, opt := range opts {
		opt(ph)
	}

	ph.peerHandlerCollector = metrics.NewPeerHandlerCollector("blocktx", ph.stats)
	metrics.Register(ph.peerHandlerCollector)

	return ph, nil
}

func (ph *PeerHandler) Start() {
	ph.startPeerWorker()
	ph.startProcessTxs()
	ph.startProcessRequestTxs()
}

func (ph *PeerHandler) startPeerWorker() {
	ph.quitPeerWorker = make(chan struct{})
	ph.quitPeerWorkerComplete = make(chan struct{})

	go func() {
		defer func() {
			ph.quitPeerWorkerComplete <- struct{}{}
		}()
		for {
			select {
			case <-ph.quitPeerWorker:
				return
			case workerItem := <-ph.workerCh:
				hash := workerItem.Hash
				peer := workerItem.Peer

				ctx := context.Background()

				bhs, err := ph.store.GetBlockHashesProcessingInProgress(ctx, ph.hostname)
				if err != nil {
					ph.logger.Error("failed to get block hashes where processing in progress", slog.String("err", err.Error()))
				}

				if len(bhs) >= maxBlocksInProgress {
					ph.logger.Debug("max blocks being processed reached", slog.String("hash", hash.String()), slog.Int("max", maxBlocksInProgress), slog.Int("number", len(bhs)))
					continue
				}

				processedBy, err := ph.store.SetBlockProcessing(ctx, hash, ph.hostname)
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

func (ph *PeerHandler) StartFillGaps(peers []p2p.PeerI) {
	ph.quitFillBlockGap = make(chan struct{})
	ph.quitFillBlockGapComplete = make(chan struct{})

	go func() {
		defer func() {
			ph.quitFillBlockGapComplete <- struct{}{}
		}()

		peerIndex := 0
		for {
			select {
			case <-ph.quitFillBlockGap:
				return
			case <-ph.fillGapsTicker.C:
				if peerIndex >= len(peers) {
					peerIndex = 0
				}

				err := ph.FillGaps(peers[peerIndex])
				if err != nil {
					ph.logger.Error("failed to fill gaps", slog.String("error", err.Error()))
				}

				peerIndex++
			}
		}
	}()
}

func (ph *PeerHandler) startProcessTxs() {
	ph.quitListenTxChannel = make(chan struct{})
	ph.quitListenTxChannelComplete = make(chan struct{})
	txHashes := make([]*blocktx_api.TransactionAndSource, 0, ph.registerTxsBatchSize)

	ticker := time.NewTicker(ph.registerTxsInterval)
	go func() {
		defer func() {
			ph.quitListenTxChannelComplete <- struct{}{}
		}()

		for {
			select {
			case <-ph.quitListenTxChannel:
				return
			case txHash := <-ph.txChannel:
				txHashes = append(txHashes, &blocktx_api.TransactionAndSource{
					Hash: txHash,
				})

				if len(txHashes) < ph.registerTxsBatchSize {
					continue
				}
				err := ph.store.RegisterTransactions(context.Background(), txHashes)
				if err != nil {
					ph.logger.Error("failed to register transactions", slog.String("err", err.Error()))
				}
				txHashes = make([]*blocktx_api.TransactionAndSource, 0, ph.registerTxsBatchSize)

			case <-ticker.C:
				if len(txHashes) == 0 {
					continue
				}
				err := ph.store.RegisterTransactions(context.Background(), txHashes)
				if err != nil {
					ph.logger.Error("failed to register transactions", slog.String("err", err.Error()))
				}
				txHashes = make([]*blocktx_api.TransactionAndSource, 0, ph.registerTxsBatchSize)
			}
		}
	}()
}

func (ph *PeerHandler) startProcessRequestTxs() {
	ctx, cancel := context.WithCancel(context.Background())
	ph.cancelListenRequestTxChannel = cancel
	ph.quitListenRequestTxChannelComplete = make(chan struct{})

	txHashes := make([]*chainhash.Hash, 0, ph.registerRequestTxsBatchSize)

	ticker := time.NewTicker(ph.registerRequestTxsInterval)

	go func() {
		defer func() {
			ph.quitListenRequestTxChannelComplete <- struct{}{}
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case txHash := <-ph.requestTxChannel:
				tx, err := chainhash.NewHash(txHash)
				if err != nil {
					ph.logger.Error("Couldn't create tx from byte array")
					continue
				}

				txHashes = append(txHashes, tx)

				if len(txHashes) < ph.registerRequestTxsBatchSize || len(txHashes) == 0 {
					continue
				}

				err = ph.publishMinedTxs(ctx, txHashes)
				if err != nil {
					ph.logger.Error("failed to publish mined txs", slog.String("err", err.Error()))
					continue
				}

				txHashes = make([]*chainhash.Hash, 0, ph.registerRequestTxsBatchSize)

			case <-ticker.C:
				if len(txHashes) == 0 {
					continue
				}

				err := ph.publishMinedTxs(ctx, txHashes)
				if err != nil {
					ph.logger.Error("failed to publish mined txs", slog.String("err", err.Error()))
					continue
				}

				txHashes = make([]*chainhash.Hash, 0, ph.registerRequestTxsBatchSize)
			}
		}
	}()
}

func (ph *PeerHandler) publishMinedTxs(ctx context.Context, txHashes []*chainhash.Hash) error {
	minedTxs, err := ph.store.GetMinedTransactions(ctx, txHashes)
	if err != nil {
		return fmt.Errorf("failed to get mined transactions: %v", err)
	}

	updatesBatch := make([]*blocktx_api.TransactionBlock, 0, ph.registerRequestTxsBatchSize)
	for _, minedTx := range minedTxs {
		updatesBatch = append(updatesBatch, &blocktx_api.TransactionBlock{
			TransactionHash: minedTx.TxHash,
			BlockHash:       minedTx.BlockHash,
			BlockHeight:     minedTx.BlockHeight,
			MerklePath:      minedTx.MerklePath,
		})
	}

	err = ph.mqClient.PublishMinedTxs(ctx, updatesBatch)
	if err != nil {
		return fmt.Errorf("failed to publish mined transactions: %v", err)
	}

	return nil
}

func (ph *PeerHandler) HandleTransactionGet(_ *wire.InvVect, peer p2p.PeerI) ([]byte, error) {
	peerStr := peer.String()

	stat, ok := ph.stats.Get(peerStr)
	if !ok {
		stat = &metrics.PeerHandlerStats{}
		ph.stats.Set(peerStr, stat)
	}

	stat.TransactionGet.Add(1)

	return nil, nil
}

func (ph *PeerHandler) HandleTransactionSent(_ *wire.MsgTx, peer p2p.PeerI) error {
	peerStr := peer.String()

	stat, ok := ph.stats.Get(peerStr)
	if !ok {
		stat = &metrics.PeerHandlerStats{}
		ph.stats.Set(peerStr, stat)
	}

	stat.TransactionSent.Add(1)

	return nil
}

func (ph *PeerHandler) HandleTransactionAnnouncement(_ *wire.InvVect, peer p2p.PeerI) error {
	peerStr := peer.String()

	stat, ok := ph.stats.Get(peerStr)
	if !ok {
		stat = &metrics.PeerHandlerStats{}
		ph.stats.Set(peerStr, stat)
	}

	stat.TransactionAnnouncement.Add(1)

	return nil
}

func (ph *PeerHandler) HandleTransactionRejection(_ *wire.MsgReject, peer p2p.PeerI) error {
	peerStr := peer.String()

	stat, ok := ph.stats.Get(peerStr)
	if !ok {
		stat = &metrics.PeerHandlerStats{}
		ph.stats.Set(peerStr, stat)
	}

	stat.TransactionRejection.Add(1)

	return nil
}

func (ph *PeerHandler) HandleTransaction(msg *wire.MsgTx, peer p2p.PeerI) error {
	peerStr := peer.String()

	stat, ok := ph.stats.Get(peerStr)
	if !ok {
		stat = &metrics.PeerHandlerStats{}
		ph.stats.Set(peerStr, stat)
	}

	stat.Transaction.Add(1)

	return nil
}

func (ph *PeerHandler) HandleBlockAnnouncement(msg *wire.InvVect, peer p2p.PeerI) error {

	peerStr := peer.String()

	stat, ok := ph.stats.Get(peerStr)
	if !ok {
		stat = &metrics.PeerHandlerStats{}
		ph.stats.Set(peerStr, stat)
	}

	stat.BlockAnnouncement.Add(1)

	pair := hashPeer{
		Hash: &msg.Hash,
		Peer: peer,
	}

	ph.workerCh <- pair

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

func (ph *PeerHandler) HandleBlock(wireMsg wire.Message, peer p2p.PeerI) error {
	ctx := context.Background()

	if tracer != nil {
		var span trace.Span
		ctx, span = tracer.Start(ctx, "HandleBlock")
		defer span.End()
	}
	peerStr := peer.String()

	stat, ok := ph.stats.Get(peerStr)
	if !ok {
		stat = &metrics.PeerHandlerStats{}
		ph.stats.Set(peerStr, stat)
	}

	stat.Block.Add(1)

	timeStart := time.Now()

	msg, ok := wireMsg.(*p2p.BlockMessage)
	if !ok {
		return fmt.Errorf("unable to cast wire.Message to p2p.BlockMessage")
	}

	blockHash := msg.Header.BlockHash()

	previousBlockHash := msg.Header.PrevBlock

	merkleRoot := msg.Header.MerkleRoot

	blockId, err := ph.insertBlock(ctx, &blockHash, &merkleRoot, &previousBlockHash, msg.Height)
	if err != nil {
		errDel := ph.store.DelBlockProcessing(ctx, &blockHash, ph.hostname)
		if errDel != nil {
			ph.logger.Error("failed to delete block processing - after inserting block failed", slog.String("hash", blockHash.String()), slog.String("err", errDel.Error()))
		}
		return fmt.Errorf("unable to insert block %s at height %d: %v", blockHash.String(), msg.Height, err)
	}

	calculatedMerkleTree := buildMerkleTreeStoreChainHash(ctx, msg.TransactionHashes)

	if !merkleRoot.IsEqual(calculatedMerkleTree[len(calculatedMerkleTree)-1]) {
		errDel := ph.store.DelBlockProcessing(ctx, &blockHash, ph.hostname)
		if errDel != nil {
			ph.logger.Error("failed to delete block processing - after merkle root mismatch", slog.String("hash", blockHash.String()), slog.String("err", errDel.Error()))
		}
		return fmt.Errorf("merkle root mismatch for block %s", blockHash.String())
	}

	if err = ph.markTransactionsAsMined(ctx, blockId, calculatedMerkleTree, msg.Height, &blockHash); err != nil {
		errDel := ph.store.DelBlockProcessing(ctx, &blockHash, ph.hostname)
		if errDel != nil {
			ph.logger.Error("failed to delete block processing - after marking transactions as mined failed", slog.String("hash", blockHash.String()), slog.String("err", errDel.Error()))
		}
		return fmt.Errorf("unable to mark block as mined %s: %v", blockHash.String(), err)
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
		errDel := ph.store.DelBlockProcessing(ctx, &blockHash, ph.hostname)
		if errDel != nil {
			ph.logger.Error("failed to delete block processing - after marking block as processed failed", slog.String("hash", blockHash.String()), slog.String("err", errDel.Error()))
		}
		return fmt.Errorf("unable to mark block as processed %s: %v", blockHash.String(), err)
	}

	// add the total block processing time to the stats
	stat.BlockProcessingMs.Add(uint64(time.Since(timeStart).Milliseconds()))
	ph.logger.Info("Processed block", slog.String("hash", blockHash.String()), slog.Int("txs", len(msg.TransactionHashes)), slog.String("duration", time.Since(timeStart).String()))

	return nil
}

const (
	hoursPerDay   = 24
	blocksPerHour = 6
)

func (ph *PeerHandler) FillGaps(peer p2p.PeerI) error {

	heightRange := ph.dataRetentionDays * hoursPerDay * blocksPerHour

	blockHeightGaps, err := ph.store.GetBlockGaps(context.Background(), heightRange)
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

		pair := hashPeer{
			Hash: gaps.Hash,
			Peer: peer,
		}
		ph.workerCh <- pair
	}

	return nil
}

func (ph *PeerHandler) insertBlock(ctx context.Context, blockHash *chainhash.Hash, merkleRoot *chainhash.Hash, previousBlockHash *chainhash.Hash, height uint64) (uint64, error) {

	ph.logger.Info("Inserting block", slog.String("hash", blockHash.String()), slog.Int64("height", int64(height)))

	block := &blocktx_api.Block{
		Hash:         blockHash[:],
		MerkleRoot:   merkleRoot[:],
		PreviousHash: previousBlockHash[:],
		Height:       height,
	}

	return ph.store.InsertBlock(ctx, block)
}

func (ph *PeerHandler) printMemStats() {
	bToMb := func(b uint64) uint64 {
		return b / 1024 / 1024
	}
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	ph.logger.Debug("stats",
		slog.Uint64("Alloc [MiB]", bToMb(mem.Alloc)),
		slog.Uint64("TotalAlloc [MiB]", bToMb(mem.TotalAlloc)),
		slog.Uint64("Sys [MiB]", bToMb(mem.Sys)),
		slog.Int64("NumGC [MiB]", int64(mem.NumGC)),
	)
}

func (ph *PeerHandler) markTransactionsAsMined(ctx context.Context, blockId uint64, merkleTree []*chainhash.Hash, blockHeight uint64, blockhash *chainhash.Hash) error {
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
			updateResp, err := ph.store.UpdateBlockTransactions(ctx, blockId, txs, merklePaths)
			if err != nil {
				return fmt.Errorf("failed to insert block transactions at block height %d: %v", blockHeight, err)
			}
			// free up memory
			txs = make([]*blocktx_api.TransactionAndSource, 0, ph.transactionStorageBatchSize)
			merklePaths = make([]string, 0, ph.transactionStorageBatchSize)

			updatesBatch := make([]*blocktx_api.TransactionBlock, len(updateResp))

			for i, updResp := range updateResp {
				updatesBatch[i] = &blocktx_api.TransactionBlock{
					TransactionHash: updResp.TxHash[:],
					BlockHash:       blockhash[:],
					BlockHeight:     blockHeight,
					MerklePath:      updResp.MerklePath,
				}
			}

			if len(updatesBatch) > 0 {
				err = ph.mqClient.PublishMinedTxs(ctx, updatesBatch)
				if err != nil {
					ph.logger.Error("failed to publish mined txs", slog.String("hash", blockhash.String()), slog.Int64("height", int64(blockHeight)), slog.String("err", err.Error()))
				}
			}

			// print stats, call gc and check the result
			ph.printMemStats()
			runtime.GC()
			ph.printMemStats()
		}
	}

	if iterateMerkleTree != nil {
		iterateMerkleTree.End()
	}

	// update all remaining transactions
	updateResp, err := ph.store.UpdateBlockTransactions(ctx, blockId, txs, merklePaths)
	if err != nil {
		return fmt.Errorf("failed to insert block transactions at block height %d: %v", blockHeight, err)
	}

	updatesBatch := make([]*blocktx_api.TransactionBlock, len(updateResp))

	for i, updResp := range updateResp {
		updatesBatch[i] = &blocktx_api.TransactionBlock{
			TransactionHash: updResp.TxHash[:],
			BlockHash:       blockhash[:],
			BlockHeight:     blockHeight,
			MerklePath:      updResp.MerklePath,
		}
	}

	if len(updatesBatch) > 0 {
		err = ph.mqClient.PublishMinedTxs(ctx, updatesBatch)
		if err != nil {
			ph.logger.Error("failed to publish mined txs", slog.String("err", err.Error()))
		}
	}

	return nil
}

func (ph *PeerHandler) markBlockAsProcessed(ctx context.Context, block *p2p.Block) error {
	err := ph.store.MarkBlockAsDone(ctx, block.Hash, block.Size, block.TxCount)
	if err != nil {
		return err
	}

	return nil
}

func extractHeightFromCoinbaseTx(tx *bt.Tx) uint64 {
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

func (ph *PeerHandler) Shutdown() {
	if ph.quitFillBlockGap != nil {
		ph.quitFillBlockGap <- struct{}{}
		<-ph.quitFillBlockGapComplete
		ph.fillGapsTicker.Stop()
	}

	if ph.quitPeerWorker != nil {
		ph.quitPeerWorker <- struct{}{}
		<-ph.quitPeerWorkerComplete
	}

	if ph.quitListenTxChannel != nil {
		ph.quitListenTxChannel <- struct{}{}
		<-ph.quitListenTxChannelComplete
	}

	if ph.cancelListenRequestTxChannel != nil {
		ph.cancelListenRequestTxChannel()
		<-ph.quitListenRequestTxChannelComplete
	}
	ph.unregisterTracing()
}

func (ph *PeerHandler) unregisterTracing() {
	metrics.Unregister(ph.peerHandlerCollector)
}
