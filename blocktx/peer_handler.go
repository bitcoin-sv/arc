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
	blockCh                     chan *blocktx_api.Block
	store                       store.Interface
	logger                      *slog.Logger
	announcedCache              *expiringmap.ExpiringMap[chainhash.Hash, []p2p.PeerI]
	stats                       *safemap.Safemap[string, *tracing.PeerHandlerStats]
	transactionStorageBatchSize int
	peerHandlerCollector        *tracing.PeerHandlerCollector
	startingHeight              int
}

func init() {
	gocore.NewStat("blocktx", true).NewStat("HandleBlock", true)
}

func WithTransactionBatchSize(size int) func(handler *PeerHandler) {
	return func(p *PeerHandler) {
		p.transactionStorageBatchSize = size
	}
}

func NewPeerHandler(logger *slog.Logger, storeI store.Interface, blockCh chan *blocktx_api.Block, startingHeight int, opts ...func(*PeerHandler)) *PeerHandler {
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

	s := &PeerHandler{
		store:                       storeI,
		blockCh:                     blockCh,
		logger:                      logger,
		workerCh:                    make(chan utils.Pair[*chainhash.Hash, p2p.PeerI], 100),
		announcedCache:              expiringmap.New[chainhash.Hash, []p2p.PeerI](10 * time.Minute).WithEvictionFunction(evictionFunc),
		stats:                       safemap.New[string, *tracing.PeerHandlerStats](),
		transactionStorageBatchSize: transactionStoringBatchsizeDefault,
		startingHeight:              startingHeight,
	}

	for _, opt := range opts {
		opt(s)
	}

	s.peerHandlerCollector = tracing.NewPeerHandlerCollector("blocktx", s.stats)
	tracing.Register(s.peerHandlerCollector)

	go func() {
		for pair := range s.workerCh {
			hash := pair.First
			peer := pair.Second

			item, found := s.announcedCache.Get(*hash)
			if !found {
				s.announcedCache.Set(*hash, []p2p.PeerI{peer})
				logger.Debug("added block hash with peer to announced cache", slog.String("hash", hash.String()), slog.String("peer", peer.String()))
			} else {
				// if already was announced to peer, continue
				for _, announcedPeer := range item {
					if announcedPeer.String() == peer.String() {
						continue
					}
				}

				item = append(item, peer)
				s.announcedCache.Set(*hash, item)
				logger.Debug("added peer to announced cache of block hash", slog.String("hash", hash.String()), slog.String("peer", peer.String()))
				continue
			}

			msg := wire.NewMsgGetData()
			if err := msg.AddInvVect(wire.NewInvVect(wire.InvTypeBlock, hash)); err != nil {
				logger.Error("ProcessBlock: could not create InvVect", slog.String("err", err.Error()))
				continue
			}

			if err := peer.WriteMsg(msg); err != nil {
				logger.Error("ProcessBlock: failed to write message to peer", slog.String("err", err.Error()))
				continue
			}

			logger.Info("ProcessBlock", slog.String("hash", hash.String()))
		}
	}()

	return s
}

func (bs *PeerHandler) HandleTransactionGet(_ *wire.InvVect, peer p2p.PeerI) ([]byte, error) {
	peerStr := peer.String()

	stat, ok := bs.stats.Get(peerStr)
	if !ok {
		stat = &tracing.PeerHandlerStats{}
		bs.stats.Set(peerStr, stat)
	}

	stat.TransactionGet.Add(1)

	return nil, nil
}

func (bs *PeerHandler) HandleTransactionSent(_ *wire.MsgTx, peer p2p.PeerI) error {
	peerStr := peer.String()

	stat, ok := bs.stats.Get(peerStr)
	if !ok {
		stat = &tracing.PeerHandlerStats{}
		bs.stats.Set(peerStr, stat)
	}

	stat.TransactionSent.Add(1)

	return nil
}

func (bs *PeerHandler) HandleTransactionAnnouncement(_ *wire.InvVect, peer p2p.PeerI) error {
	peerStr := peer.String()

	stat, ok := bs.stats.Get(peerStr)
	if !ok {
		stat = &tracing.PeerHandlerStats{}
		bs.stats.Set(peerStr, stat)
	}

	stat.TransactionAnnouncement.Add(1)

	return nil
}

func (bs *PeerHandler) HandleTransactionRejection(_ *wire.MsgReject, peer p2p.PeerI) error {
	peerStr := peer.String()

	stat, ok := bs.stats.Get(peerStr)
	if !ok {
		stat = &tracing.PeerHandlerStats{}
		bs.stats.Set(peerStr, stat)
	}

	stat.TransactionRejection.Add(1)

	return nil
}

func (bs *PeerHandler) HandleTransaction(msg *wire.MsgTx, peer p2p.PeerI) error {
	peerStr := peer.String()

	stat, ok := bs.stats.Get(peerStr)
	if !ok {
		stat = &tracing.PeerHandlerStats{}
		bs.stats.Set(peerStr, stat)
	}

	stat.Transaction.Add(1)

	return nil
}

func (bs *PeerHandler) CheckPrimary() (bool, error) {
	primaryBlocktx, err := bs.store.PrimaryBlocktx(context.TODO())
	if err != nil {
		return false, err
	}

	hostName, err := os.Hostname()
	if err != nil {
		return false, err
	}

	if primaryBlocktx != hostName {
		bs.logger.Info("Not primary, skipping block processing")
		return false, nil
	}

	return true, nil
}

func (bs *PeerHandler) HandleBlockAnnouncement(msg *wire.InvVect, peer p2p.PeerI) error {
	primary, err := bs.CheckPrimary()
	if err != nil {
		return err
	}

	if !primary {
		return nil
	}

	peerStr := peer.String()

	stat, ok := bs.stats.Get(peerStr)
	if !ok {
		stat = &tracing.PeerHandlerStats{}
		bs.stats.Set(peerStr, stat)
	}

	stat.BlockAnnouncement.Add(1)

	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("HandleBlockAnnouncement").AddTime(start)
	}()

	pair := utils.NewPair(&msg.Hash, peer)
	utils.SafeSend(bs.workerCh, pair)

	return nil
}

func (bs *PeerHandler) HandleBlock(wireMsg wire.Message, peer p2p.PeerI) error {
	peerStr := peer.String()

	stat, ok := bs.stats.Get(peerStr)
	if !ok {
		stat = &tracing.PeerHandlerStats{}
		bs.stats.Set(peerStr, stat)
	}

	stat.Block.Add(1)

	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("HandleBlock").AddTime(start)
	}()

	primary, err := bs.CheckPrimary()
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

	blockId, err := bs.insertBlock(&blockHash, &merkleRoot, &previousBlockHash, msg.Height, peer)
	if err != nil {
		return fmt.Errorf("unable to insert block %s: %v", blockHash.String(), err)
	}

	calculatedMerkleTree := bc.BuildMerkleTreeStoreChainHash(msg.TransactionHashes)

	if !merkleRoot.IsEqual(calculatedMerkleTree[len(calculatedMerkleTree)-1]) {
		return fmt.Errorf("merkle root mismatch for block %s", blockHash.String())
	}

	if err = bs.markTransactionsAsMined(blockId, calculatedMerkleTree, msg.Height); err != nil {
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

	if err = bs.markBlockAsProcessed(block); err != nil {
		return fmt.Errorf("unable to mark block as processed %s: %v", blockHash.String(), err)
	}

	// add the total block processing time to the stats
	stat.BlockProcessingMs.Add(uint64(time.Since(timeStart).Milliseconds()))
	bs.logger.Info("Processed block", slog.String("hash", blockHash.String()), slog.Int("txs", len(msg.TransactionHashes)), slog.String("duration", time.Since(timeStart).String()))

	return nil
}

func (bs *PeerHandler) FillGaps(peer p2p.PeerI) error {
	primary, err := bs.CheckPrimary()
	if err != nil {
		return err
	}

	if !primary {
		return nil
	}

	blockHeightGaps, err := bs.store.GetBlockGaps(context.Background())
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

		_, found := bs.announcedCache.Get(*gaps.Hash)
		if found {
			return nil
		}

		bs.logger.Info("requesting missing block", slog.String("hash", gaps.Hash.String()), slog.Int64("height", int64(gaps.Height)))

		pair := utils.NewPair(gaps.Hash, peer)
		utils.SafeSend(bs.workerCh, pair)
	}

	return nil
}

func (bs *PeerHandler) insertBlock(blockHash *chainhash.Hash, merkleRoot *chainhash.Hash, previousBlockHash *chainhash.Hash, height uint64, peer p2p.PeerI) (uint64, error) {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("HandleBlock").NewStat("insertBlock").AddTime(start)
	}()

	if height > uint64(bs.startingHeight) {
		if _, found := bs.announcedCache.Get(*previousBlockHash); !found {
			if _, err := bs.store.GetBlock(context.Background(), previousBlockHash); err != nil {
				if errors.Is(err, store.ErrBlockNotFound) {
					pair := utils.NewPair(previousBlockHash, peer)
					utils.SafeSend(bs.workerCh, pair)
				} else if err != nil {
					bs.logger.Error("failed to get previous block", slog.String("hash", previousBlockHash.String()), slog.Int64("height", int64(height-1)), slog.String("err", err.Error()))
				}
			}
		}
	}

	bs.logger.Info("inserting block", slog.String("hash", blockHash.String()), slog.Int64("height", int64(height)))

	block := &blocktx_api.Block{
		Hash:         blockHash[:],
		MerkleRoot:   merkleRoot[:],
		PreviousHash: previousBlockHash[:],
		Height:       height,
	}

	return bs.store.InsertBlock(context.Background(), block)
}

func printMemStats() {
	bToMb := func(b uint64) uint64 {
		return b / 1024 / 1024
	}
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	fmt.Printf("Alloc = %v MiB, TotalAlloc = %v MiB, Sys = %v MiB, NumGC = %v\n",
		bToMb(mem.Alloc), bToMb(mem.TotalAlloc), bToMb(mem.Sys), mem.NumGC)
}

func (bs *PeerHandler) markTransactionsAsMined(blockId uint64, merkleTree []*chainhash.Hash, blockHeight uint64) error {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("HandleBlock").NewStat("markTransactionsAsMined").AddTime(start)
	}()

	txs := make([]*blocktx_api.TransactionAndSource, 0, bs.transactionStorageBatchSize)
	merklePaths := make([]string, 0, bs.transactionStorageBatchSize)
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
		if (txIndex+1)%bs.transactionStorageBatchSize == 0 {
			if err := bs.store.InsertBlockTransactions(context.Background(), blockId, txs, merklePaths); err != nil {
				return fmt.Errorf("failed to insert block transactions at block height %d: %v", blockHeight, err)
			}
			// free up memory
			txs = txs[:0]
			merklePaths = merklePaths[:0]

			// print stats, call gc and chec the result
			printMemStats()
			runtime.GC()
			printMemStats()
		}
	}

	// insert all remaining transactions into the table
	if err := bs.store.InsertBlockTransactions(context.Background(), blockId, txs, merklePaths); err != nil {
		return fmt.Errorf("failed to insert block transactions at block height %d: %v", blockHeight, err)
	}

	return nil
}

func (bs *PeerHandler) getAnnouncedCacheBlockHashes() []string {
	items := bs.announcedCache.Items()
	blockHashes := make([]string, len(items))
	i := 0
	for k := range items {
		blockHashes[i] = k.String()
		i++
	}

	return blockHashes
}

func (bs *PeerHandler) markBlockAsProcessed(block *p2p.Block) error {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("HandleBlock").NewStat("markBlockAsProcessed").AddTime(start)
	}()

	err := bs.store.MarkBlockAsDone(context.Background(), block.Hash, block.Size, block.TxCount)
	if err != nil {
		return err
	}

	bs.announcedCache.Delete(*block.Hash)
	bs.logger.Debug("removed block from announced cache", slog.String("hash", block.Hash.String()))

	utils.SafeSend(bs.blockCh, &blocktx_api.Block{
		Hash:         block.Hash[:],
		PreviousHash: block.PreviousHash[:],
		MerkleRoot:   block.MerkleRoot[:],
		Height:       block.Height,
	})

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

func (bs *PeerHandler) Shutdown() {
	tracing.Unregister(bs.peerHandlerCollector)
}
