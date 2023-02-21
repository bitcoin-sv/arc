package blocktx

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math/rand"
	"time"

	"github.com/TAAL-GmbH/arc/blocktx/blocktx_api"
	"github.com/TAAL-GmbH/arc/blocktx/store"
	"github.com/TAAL-GmbH/arc/tracing"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/blockchain"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/go-utils/expiringmap"
	"github.com/ordishs/go-utils/safemap"
	"github.com/ordishs/gocore"
)

type PeerHandler struct {
	workerCh       chan utils.Pair[[]byte, p2p.PeerI]
	blockCh        chan *blocktx_api.Block
	store          store.Interface
	logger         utils.Logger
	announcedCache *expiringmap.ExpiringMap[string, []p2p.PeerI]
	stats          *safemap.Safemap[string, *tracing.PeerHandlerStats]
}

func init() {
	gocore.NewStat("blocktx", true).NewStat("HandleBlock", true)
}

func NewPeerHandler(logger utils.Logger, storeI store.Interface, blockCh chan *blocktx_api.Block) p2p.PeerHandlerI {
	s := &PeerHandler{
		store:    storeI,
		blockCh:  blockCh,
		logger:   logger,
		workerCh: make(chan utils.Pair[[]byte, p2p.PeerI], 100),
		announcedCache: expiringmap.New[string, []p2p.PeerI](5 * time.Minute).WithEvictionFunction(func(hashStr string, peers []p2p.PeerI) bool {
			msg := wire.NewMsgGetData()
			hash, err := chainhash.NewHashFromStr(hashStr)
			if err != nil {
				logger.Errorf("EvictionFunc: invalid hash: %v", err)
				return false
			}

			if err := msg.AddInvVect(wire.NewInvVect(wire.InvTypeBlock, hash)); err != nil {
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
		}),
		stats: safemap.New[string, *tracing.PeerHandlerStats](),
	}

	_ = tracing.NewPeerHandlerCollector("blocktx", s.stats)

	go func() {
		for pair := range s.workerCh {
			hash, err := chainhash.NewHash(pair.First)
			if err != nil {
				logger.Errorf("ProcessBlock: invalid hash %s: %v", hash, err)
				continue
			}

			peer := pair.Second

			id := hash.String()
			item, found := s.announcedCache.Get(id)
			if !found {
				s.announcedCache.Set(id, []p2p.PeerI{peer})

			} else {
				item = append(item, peer)
				s.announcedCache.Set(id, item)
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

func (bs *PeerHandler) HandleBlockAnnouncement(msg *wire.InvVect, peer p2p.PeerI) error {
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

	pair := utils.NewPair(msg.Hash.CloneBytes(), peer)
	utils.SafeSend(bs.workerCh, pair)

	return nil
}

func (bs *PeerHandler) HandleBlock(msg *p2p.BlockMessage, peer p2p.PeerI) error {
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

	timeStart := time.Now()

	blockHash := msg.Header.BlockHash()
	blockHashBytes := blockHash.CloneBytes()

	previousBlockHash := msg.Header.PrevBlock
	previousBlockHashBytes := previousBlockHash.CloneBytes()

	merkleRoot := msg.Header.MerkleRoot
	merkleRootBytes := merkleRoot.CloneBytes()

	blockId, err := bs.insertBlock(blockHashBytes, merkleRootBytes, previousBlockHashBytes, msg.Height, peer)
	if err != nil {
		return fmt.Errorf("unable to insert block %s: %v", blockHash.String(), err)
	}

	if err = bs.markTransactionsAsMined(blockId, msg.TransactionIDs); err != nil {
		return fmt.Errorf("unable to mark block as mined %s: %v", blockHash.String(), err)
	}

	calculatedMerkleRoot := blockchain.BuildMerkleTreeStore(msg.TransactionIDs)
	if !bytes.Equal(calculatedMerkleRoot[len(calculatedMerkleRoot)-1], merkleRootBytes) {
		return fmt.Errorf("merkle root mismatch for block %s", blockHash.String())
	}

	block := &p2p.Block{
		Hash:         blockHashBytes,
		MerkleRoot:   merkleRootBytes,
		PreviousHash: previousBlockHashBytes,
		Height:       msg.Height,
		Size:         msg.Size,
		TxCount:      uint64(len(msg.TransactionIDs)),
	}

	if err = bs.markBlockAsProcessed(block); err != nil {
		return fmt.Errorf("unable to mark block as processed %s: %v", blockHash.String(), err)
	}

	// add the total block processing time to the stats
	stat.BlockProcessingMs.Add(uint64(time.Since(timeStart).Milliseconds()))
	bs.logger.Infof("Processed block %s, %d transactions in %0.2f seconds", blockHash.String(), len(msg.TransactionIDs), time.Since(timeStart).Seconds())

	return nil
}

func (bs *PeerHandler) insertBlock(blockHash []byte, merkleRoot []byte, previousBlockHash []byte, height uint64, peer p2p.PeerI) (uint64, error) {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("HandleBlock").NewStat("insertBlock").AddTime(start)
	}()

	startingHeight, _ := gocore.Config().GetInt("starting_block_height", 700000)
	if height > uint64(startingHeight) {
		if _, found := bs.announcedCache.Get(utils.HexEncodeAndReverseBytes(previousBlockHash)); !found {
			if _, err := bs.store.GetBlock(context.Background(), previousBlockHash); err != nil {
				if err == sql.ErrNoRows {
					pair := utils.NewPair(previousBlockHash, peer)
					utils.SafeSend(bs.workerCh, pair)
				}
			}
		}
	}

	block := &blocktx_api.Block{
		Hash:         blockHash,
		MerkleRoot:   merkleRoot,
		PreviousHash: previousBlockHash,
		Height:       height,
	}

	return bs.store.InsertBlock(context.Background(), block)
}

func (bs *PeerHandler) markTransactionsAsMined(blockId uint64, transactions [][]byte) error {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("HandleBlock").NewStat("markTransactionsAsMined").AddTime(start)
	}()

	txs := make([]*blocktx_api.TransactionAndSource, 0, len(transactions))

	for _, tx := range transactions {
		txs = append(txs, &blocktx_api.TransactionAndSource{
			Hash: tx,
		})
	}

	if err := bs.store.InsertBlockTransactions(context.Background(), blockId, txs); err != nil {
		return err
	}

	return nil
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

	bs.announcedCache.Delete(utils.HexEncodeAndReverseBytes(block.Hash))

	utils.SafeSend(bs.blockCh, &blocktx_api.Block{
		Hash:         block.Hash,
		PreviousHash: block.PreviousHash,
		MerkleRoot:   block.MerkleRoot,
		Height:       block.Height,
	})

	return nil
}

func extractHeightFromCoinbaseTx(tx *wire.MsgTx) uint64 {
	// Coinbase tx has a special format, the height is encoded in the first 4 bytes of the scriptSig
	// https://en.bitcoin.it/wiki/Protocol_documentation#tx
	// Get the length
	length := int(tx.TxIn[0].SignatureScript[0])

	if len(tx.TxIn[0].SignatureScript) < length+1 {
		return 0
	}

	b := make([]byte, 8)

	for i := 0; i < length; i++ {
		b[i] = tx.TxIn[0].SignatureScript[i+1]
	}

	return binary.LittleEndian.Uint64(b)
}
