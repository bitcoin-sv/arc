package blocktx

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"os"
	"runtime"
	"time"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/blocktx/store"
	"github.com/bitcoin-sv/arc/tracing"
	"github.com/libsv/go-bc"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/go-utils/expiringmap"
	"github.com/ordishs/go-utils/safemap"
	"github.com/ordishs/gocore"
)

const (
	transactionStoringBatchsizeDefault = 2048 // power of 2 for easier memory allocation
	maxRequestBlocks                   = 5
	fillGapsInterval                   = 15 * time.Minute
	maximumBlockSize                   = 4294967296 // 4Gb
	registerTxsIntervalDefault         = time.Second * 10
	registerTxsBatchSizeDefault        = 100
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

type PeerHandler struct {
	workerCh                    chan utils.Pair[*chainhash.Hash, p2p.PeerI]
	store                       store.Interface
	logger                      *slog.Logger
	announcedCache              *expiringmap.ExpiringMap[chainhash.Hash, []p2p.PeerI]
	stats                       *safemap.Safemap[string, *tracing.PeerHandlerStats]
	transactionStorageBatchSize int
	peerHandlerCollector        *tracing.PeerHandlerCollector
	startingHeight              int
	dataRetentionDays           int
	mqClient                    MessageQueueClient
	txChannel                   chan []byte
	registerTxsInterval         time.Duration
	registerTxsBatchSize        int
	peers                       []*p2p.Peer

	fillGapsTicker              *time.Ticker
	quitFillBlockGap            chan struct{}
	quitFillBlockGapComplete    chan struct{}
	quitListenTxChannel         chan struct{}
	quitListenTxChannelComplete chan struct{}
}

func init() {
	gocore.NewStat("blocktx", true).NewStat("HandleBlock", true)
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

func WithTxChan(txChannel chan []byte) func(handler *PeerHandler) {
	return func(handler *PeerHandler) {
		handler.txChannel = txChannel
	}
}

func WithRegisterTxsBatchSize(size int) func(handler *PeerHandler) {
	return func(handler *PeerHandler) {
		handler.registerTxsBatchSize = size
	}
}

func NewPeerHandler(logger *slog.Logger, storeI store.Interface, startingHeight int, peerURLs []string, network wire.BitcoinNet, opts ...func(*PeerHandler)) (*PeerHandler, error) {
	evictionFunc := func(hash chainhash.Hash, peers []p2p.PeerI) bool {
		msg := wire.NewMsgGetData()

		if err := msg.AddInvVect(wire.NewInvVect(wire.InvTypeBlock, &hash)); err != nil {
			logger.Error("EvictionFunc: could not create InvVect", slog.String("err", err.Error()))
			return false
		}
		// Select a random peer to send the request to
		peer := peers[rand.Intn(len(peers))]

		if err := peer.WriteMsg(msg); err != nil {
			logger.Error("EvictionFunc: failed to write message to peer", slog.String("err", err.Error()))
			return false
		}

		logger.Info("EvictionFunc: sent block request to peer", slog.String("hash", hash.String()), slog.String("peer", peer.String()))

		return false
	}

	ph := &PeerHandler{
		store:                       storeI,
		logger:                      logger,
		workerCh:                    make(chan utils.Pair[*chainhash.Hash, p2p.PeerI], 100),
		announcedCache:              expiringmap.New[chainhash.Hash, []p2p.PeerI](10 * time.Minute).WithEvictionFunction(evictionFunc),
		stats:                       safemap.New[string, *tracing.PeerHandlerStats](),
		transactionStorageBatchSize: transactionStoringBatchsizeDefault,
		startingHeight:              startingHeight,
		registerTxsInterval:         registerTxsIntervalDefault,
		registerTxsBatchSize:        registerTxsBatchSizeDefault,
		peers:                       make([]*p2p.Peer, len(peerURLs)),

		fillGapsTicker:              time.NewTicker(fillGapsInterval),
		quitFillBlockGap:            make(chan struct{}),
		quitFillBlockGapComplete:    make(chan struct{}),
		quitListenTxChannel:         make(chan struct{}),
		quitListenTxChannelComplete: make(chan struct{}),
	}

	for _, opt := range opts {
		opt(ph)
	}

	ph.peerHandlerCollector = tracing.NewPeerHandlerCollector("blocktx", ph.stats)
	tracing.Register(ph.peerHandlerCollector)

	pm := p2p.NewPeerManager(logger, network, p2p.WithExcessiveBlockSize(maximumBlockSize))

	for i, peerURL := range peerURLs {
		peer, err := p2p.NewPeer(logger, peerURL, ph, network, p2p.WithMaximumMessageSize(maximumBlockSize))
		if err != nil {
			return nil, fmt.Errorf("error creating peer %s: %v", peerURL, err)
		}
		err = pm.AddPeer(peer)
		if err != nil {
			return nil, fmt.Errorf("error adding peer: %v", err)
		}

		ph.peers[i] = peer
	}

	ph.startFillGaps(ph.peers)
	ph.startPeerWorker()
	ph.startProcessTxs()

	return ph, nil
}

func (ph *PeerHandler) startPeerWorker() {
	go func() {
		for pair := range ph.workerCh {
			hash := pair.First
			peer := pair.Second

			item, found := ph.announcedCache.Get(*hash)
			if !found {
				ph.announcedCache.Set(*hash, []p2p.PeerI{peer})
				ph.logger.Debug("added block hash with peer to announced cache", slog.String("hash", hash.String()), slog.String("peer", peer.String()))
			} else {
				// if already was announced to peer, continue
				for _, announcedPeer := range item {
					if announcedPeer.String() == peer.String() {
						continue
					}
				}

				item = append(item, peer)
				ph.announcedCache.Set(*hash, item)
				ph.logger.Debug("added peer to announced cache of block hash", slog.String("hash", hash.String()), slog.String("peer", peer.String()))
				continue
			}

			msg := wire.NewMsgGetData()
			if err := msg.AddInvVect(wire.NewInvVect(wire.InvTypeBlock, hash)); err != nil {
				ph.logger.Error("ProcessBlock: could not create InvVect", slog.String("err", err.Error()))
				continue
			}

			if err := peer.WriteMsg(msg); err != nil {
				ph.logger.Error("ProcessBlock: failed to write message to peer", slog.String("err", err.Error()))
				continue
			}

			ph.logger.Info("ProcessBlock", slog.String("hash", hash.String()))
		}
	}()
}

func (ph *PeerHandler) startFillGaps(peers []*p2p.Peer) {
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

				ph.logger.Info("requesting missing blocks from peer", slog.Int("index", peerIndex))

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

func (ph *PeerHandler) HandleTransactionGet(_ *wire.InvVect, peer p2p.PeerI) ([]byte, error) {
	peerStr := peer.String()

	stat, ok := ph.stats.Get(peerStr)
	if !ok {
		stat = &tracing.PeerHandlerStats{}
		ph.stats.Set(peerStr, stat)
	}

	stat.TransactionGet.Add(1)

	return nil, nil
}

func (ph *PeerHandler) HandleTransactionSent(_ *wire.MsgTx, peer p2p.PeerI) error {
	peerStr := peer.String()

	stat, ok := ph.stats.Get(peerStr)
	if !ok {
		stat = &tracing.PeerHandlerStats{}
		ph.stats.Set(peerStr, stat)
	}

	stat.TransactionSent.Add(1)

	return nil
}

func (ph *PeerHandler) HandleTransactionAnnouncement(_ *wire.InvVect, peer p2p.PeerI) error {
	peerStr := peer.String()

	stat, ok := ph.stats.Get(peerStr)
	if !ok {
		stat = &tracing.PeerHandlerStats{}
		ph.stats.Set(peerStr, stat)
	}

	stat.TransactionAnnouncement.Add(1)

	return nil
}

func (ph *PeerHandler) HandleTransactionRejection(_ *wire.MsgReject, peer p2p.PeerI) error {
	peerStr := peer.String()

	stat, ok := ph.stats.Get(peerStr)
	if !ok {
		stat = &tracing.PeerHandlerStats{}
		ph.stats.Set(peerStr, stat)
	}

	stat.TransactionRejection.Add(1)

	return nil
}

func (ph *PeerHandler) HandleTransaction(msg *wire.MsgTx, peer p2p.PeerI) error {
	peerStr := peer.String()

	stat, ok := ph.stats.Get(peerStr)
	if !ok {
		stat = &tracing.PeerHandlerStats{}
		ph.stats.Set(peerStr, stat)
	}

	stat.Transaction.Add(1)

	return nil
}

func (ph *PeerHandler) CheckPrimary() (bool, error) {
	primaryBlocktx, err := ph.store.GetPrimary(context.TODO())
	if err != nil {
		return false, err
	}

	hostName, err := os.Hostname()
	if err != nil {
		return false, err
	}

	if primaryBlocktx != hostName {
		ph.logger.Info("Not primary, skipping block processing")
		return false, nil
	}

	return true, nil
}

func (ph *PeerHandler) HandleBlockAnnouncement(msg *wire.InvVect, peer p2p.PeerI) error {
	primary, err := ph.CheckPrimary()
	if err != nil {
		return err
	}

	if !primary {
		return nil
	}

	peerStr := peer.String()

	stat, ok := ph.stats.Get(peerStr)
	if !ok {
		stat = &tracing.PeerHandlerStats{}
		ph.stats.Set(peerStr, stat)
	}

	stat.BlockAnnouncement.Add(1)

	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("HandleBlockAnnouncement").AddTime(start)
	}()

	pair := utils.NewPair(&msg.Hash, peer)
	utils.SafeSend(ph.workerCh, pair)

	return nil
}

func (ph *PeerHandler) HandleBlock(wireMsg wire.Message, peer p2p.PeerI) error {
	peerStr := peer.String()

	stat, ok := ph.stats.Get(peerStr)
	if !ok {
		stat = &tracing.PeerHandlerStats{}
		ph.stats.Set(peerStr, stat)
	}

	stat.Block.Add(1)

	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("HandleBlock").AddTime(start)
	}()

	primary, err := ph.CheckPrimary()
	if err != nil {
		return err
	}

	if !primary {
		return nil
	}

	timeStart := time.Now()

	msg, ok := wireMsg.(*p2p.BlockMessage)
	if !ok {
		return fmt.Errorf("unable to cast wire.Message to p2p.BlockMessage")
	}

	blockHash := msg.Header.BlockHash()

	previousBlockHash := msg.Header.PrevBlock

	merkleRoot := msg.Header.MerkleRoot

	blockId, err := ph.insertBlock(&blockHash, &merkleRoot, &previousBlockHash, msg.Height, peer)
	if err != nil {
		return fmt.Errorf("unable to insert block %s at height %d: %v", blockHash.String(), msg.Height, err)
	}

	calculatedMerkleTree := bc.BuildMerkleTreeStoreChainHash(msg.TransactionHashes)

	if !merkleRoot.IsEqual(calculatedMerkleTree[len(calculatedMerkleTree)-1]) {
		return fmt.Errorf("merkle root mismatch for block %s", blockHash.String())
	}

	if err = ph.markTransactionsAsMined(blockId, calculatedMerkleTree, msg.Height, &blockHash); err != nil {
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

	if err = ph.markBlockAsProcessed(block); err != nil {
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
	primary, err := ph.CheckPrimary()
	if err != nil {
		return err
	}

	if !primary {
		return nil
	}

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

		_, found := ph.announcedCache.Get(*gaps.Hash)
		if found {
			return nil
		}

		ph.logger.Info("requesting missing block", slog.String("hash", gaps.Hash.String()), slog.Int64("height", int64(gaps.Height)))

		pair := utils.NewPair(gaps.Hash, peer)
		utils.SafeSend(ph.workerCh, pair)
	}

	return nil
}

func (ph *PeerHandler) insertBlock(blockHash *chainhash.Hash, merkleRoot *chainhash.Hash, previousBlockHash *chainhash.Hash, height uint64, peer p2p.PeerI) (uint64, error) {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("HandleBlock").NewStat("insertBlock").AddTime(start)
	}()

	if height > uint64(ph.startingHeight) {
		if _, found := ph.announcedCache.Get(*previousBlockHash); !found {
			if _, err := ph.store.GetBlock(context.Background(), previousBlockHash); err != nil {
				if errors.Is(err, store.ErrBlockNotFound) {
					pair := utils.NewPair(previousBlockHash, peer)
					utils.SafeSend(ph.workerCh, pair)
				} else if err != nil {
					ph.logger.Error("failed to get previous block", slog.String("hash", previousBlockHash.String()), slog.Int64("height", int64(height-1)), slog.String("err", err.Error()))
				}
			}
		}
	}

	ph.logger.Info("inserting block", slog.String("hash", blockHash.String()), slog.Int64("height", int64(height)))

	block := &blocktx_api.Block{
		Hash:         blockHash[:],
		MerkleRoot:   merkleRoot[:],
		PreviousHash: previousBlockHash[:],
		Height:       height,
	}

	return ph.store.InsertBlock(context.Background(), block)
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

func (ph *PeerHandler) markTransactionsAsMined(blockId uint64, merkleTree []*chainhash.Hash, blockHeight uint64, blockhash *chainhash.Hash) error {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("HandleBlock").NewStat("markTransactionsAsMined").AddTime(start)
	}()

	txs := make([]*blocktx_api.TransactionAndSource, 0, ph.transactionStorageBatchSize)
	merklePaths := make([]string, 0, ph.transactionStorageBatchSize)
	leaves := merkleTree[:(len(merkleTree)+1)/2]

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
		if (txIndex+1)%ph.transactionStorageBatchSize == 0 {
			updateResp, err := ph.store.UpdateBlockTransactions(context.Background(), blockId, txs, merklePaths)
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
				err = ph.mqClient.PublishMinedTxs(updatesBatch)
				if err != nil {
					ph.logger.Error("failed to publish mined txs", slog.String("err", err.Error()))
				}
			}

			// print stats, call gc and chec the result
			ph.printMemStats()
			runtime.GC()
			ph.printMemStats()
		}
	}

	// update all remaining transactions
	updateResp, err := ph.store.UpdateBlockTransactions(context.Background(), blockId, txs, merklePaths)
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
		err = ph.mqClient.PublishMinedTxs(updatesBatch)
		if err != nil {
			ph.logger.Error("failed to publish mined txs", slog.String("err", err.Error()))
		}
	}

	return nil
}

func (ph *PeerHandler) markBlockAsProcessed(block *p2p.Block) error {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("HandleBlock").NewStat("markBlockAsProcessed").AddTime(start)
	}()

	err := ph.store.MarkBlockAsDone(context.Background(), block.Hash, block.Size, block.TxCount)
	if err != nil {
		return err
	}

	ph.announcedCache.Delete(*block.Hash)
	ph.logger.Debug("removed block from announced cache", slog.String("hash", block.Hash.String()))

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
	ph.quitFillBlockGap <- struct{}{}
	<-ph.quitFillBlockGapComplete

	ph.quitListenTxChannel <- struct{}{}
	<-ph.quitListenTxChannelComplete

	ph.fillGapsTicker.Stop()
	tracing.Unregister(ph.peerHandlerCollector)
}
