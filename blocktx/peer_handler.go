package blocktx

import (
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strings"
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
	"github.com/spf13/viper"
)

const transactionStoringBatchsizeDefault = 16384 // power of 2 for easier memory allocation

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
	logger                      utils.Logger
	announcedCache              *expiringmap.ExpiringMap[chainhash.Hash, []p2p.PeerI]
	stats                       *safemap.Safemap[string, *tracing.PeerHandlerStats]
	transactionStorageBatchSize int
}

func init() {
	gocore.NewStat("blocktx", true).NewStat("HandleBlock", true)
}

func WithTransactionBatchSize(size int) func(handler *PeerHandler) {
	return func(p *PeerHandler) {
		p.transactionStorageBatchSize = size
	}
}

func NewPeerHandler(logger utils.Logger, storeI store.Interface, blockCh chan *blocktx_api.Block, opts ...func(*PeerHandler)) *PeerHandler {
	evictionFunc := func(hash chainhash.Hash, peers []p2p.PeerI) bool {
		msg := wire.NewMsgGetData()

		if err := msg.AddInvVect(wire.NewInvVect(wire.InvTypeBlock, &hash)); err != nil {
			logger.Errorf("EvictionFunc: could not create InvVect: %v", err)
			return false
		}
		// Select a random peer to send the request to
		peer := peers[rand.Intn(len(peers))]

		if err := peer.WriteMsg(msg); err != nil {
			logger.Errorf("EvictionFunc: failed to write message to peer: %v", err)
			return false
		}

		logger.Infof("EvictionFunc: sent block request %s to peer %s", hash.String(), peer.String())

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
	}

	for _, opt := range opts {
		opt(s)
	}

	_ = tracing.NewPeerHandlerCollector("blocktx", s.stats)

	go func() {
		for pair := range s.workerCh {
			hash := pair.First
			peer := pair.Second

			item, found := s.announcedCache.Get(*hash)
			if !found {
				s.announcedCache.Set(*hash, []p2p.PeerI{peer})
				logger.Debugf("added block hash %s with peer %s to announced cache", hash.String(), peer.String())

			} else {
				// if already was announced to peer, continue
				for _, announcedPeer := range item {
					if announcedPeer.String() == peer.String() {
						continue
					}
				}

				item = append(item, peer)
				s.announcedCache.Set(*hash, item)
				logger.Debugf("added peer %s to announced cache of block hash %s", peer.String(), hash.String())
				continue
			}

			msg := wire.NewMsgGetData()
			if err := msg.AddInvVect(wire.NewInvVect(wire.InvTypeBlock, hash)); err != nil {
				logger.Errorf("ProcessBlock: could not create InvVect: %v", err)
				continue
			}

			if err := peer.WriteMsg(msg); err != nil {
				logger.Errorf("ProcessBlock: failed to write message to peer: %v", err)
				continue
			}

			logger.Infof("ProcessBlock: %s", hash.String())
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
		bs.logger.Infof("Not primary, skipping block processing")
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

	calculatedMerkleTree, err := bc.BuildMerkleTreeStoreChainHash(msg.TransactionHashes)
	if err != nil {
		return err
	}

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
	bs.logger.Infof("Processed block %s, %d transactions in %0.2f seconds", blockHash.String(), len(msg.TransactionHashes), time.Since(timeStart).Seconds())

	return nil
}

func (bs *PeerHandler) insertBlock(blockHash *chainhash.Hash, merkleRoot *chainhash.Hash, previousBlockHash *chainhash.Hash, height uint64, peer p2p.PeerI) (uint64, error) {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("HandleBlock").NewStat("insertBlock").AddTime(start)
	}()

	startingHeight := viper.GetInt("blocktx.startingBlockHeight")
	if height > uint64(startingHeight) {
		if _, found := bs.announcedCache.Get(*previousBlockHash); !found {
			if _, err := bs.store.GetBlock(context.Background(), previousBlockHash); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					pair := utils.NewPair(previousBlockHash, peer)
					utils.SafeSend(bs.workerCh, pair)
				}
			}
		}
	}

	block := &blocktx_api.Block{
		Hash:         blockHash[:],
		MerkleRoot:   merkleRoot[:],
		PreviousHash: previousBlockHash[:],
		Height:       height,
	}

	return bs.store.InsertBlock(context.Background(), block)
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
			return err
		}

		bumpHex, err := bump.String()
		if err != nil {
			return err
		}

		merklePaths = append(merklePaths, bumpHex)
		if (txIndex+1)%bs.transactionStorageBatchSize == 0 {
			if err := bs.store.InsertBlockTransactions(context.Background(), blockId, txs, merklePaths); err != nil {
				return err
			}
			// free up memory
			txs = make([]*blocktx_api.TransactionAndSource, 0, bs.transactionStorageBatchSize)
			merklePaths = make([]string, 0, bs.transactionStorageBatchSize)
		}
	}

	// insert all remaining transactions into the table
	if err := bs.store.InsertBlockTransactions(context.Background(), blockId, txs, merklePaths); err != nil {
		return err
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
	bs.logger.Debugf("removed block hash %s from announced cache - remaining block hashes: %s", block.Hash.String(), strings.Join(bs.getAnnouncedCacheBlockHashes(), ","))

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
