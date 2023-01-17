package blocktx

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/TAAL-GmbH/arc/blocktx/blocktx_api"
	"github.com/TAAL-GmbH/arc/blocktx/store"
	"github.com/TAAL-GmbH/arc/p2p"
	"github.com/TAAL-GmbH/arc/p2p/blockchain"
	"github.com/TAAL-GmbH/arc/p2p/chaincfg/chainhash"
	"github.com/TAAL-GmbH/arc/p2p/wire"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/go-utils/expiringmap"
)

type PeerHandler struct {
	workerCh       chan utils.Pair[[]byte, p2p.PeerI]
	blockCh        chan *blocktx_api.Block
	store          store.Interface
	logger         utils.Logger
	announcedCache *expiringmap.ExpiringMap[string, []p2p.PeerI]
}

func NewPeerHandler(logger utils.Logger, storeI store.Interface, blockCh chan *blocktx_api.Block) p2p.PeerHandlerI {
	s := &PeerHandler{
		store:          storeI,
		blockCh:        blockCh,
		logger:         logger,
		workerCh:       make(chan utils.Pair[[]byte, p2p.PeerI], 100),
		announcedCache: expiringmap.New[string, []p2p.PeerI](5 * time.Minute),
	}

	go func() {
		for pair := range s.workerCh {
			hash, err := chainhash.NewHash(pair.First)
			if err != nil {
				logger.Errorf("ProcessBlock: %s", err)
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
				logger.Errorf("ProcessBlock: %s", err)
				continue
			}

			_ = peer.WriteMsg(msg)
			logger.Infof("ProcessBlock: %s", hash.String())
		}
	}()

	return s
}

func (bs *PeerHandler) HandleTransactionGet(_ *wire.InvVect, _ p2p.PeerI) ([]byte, error) {
	return nil, nil
}

func (bs *PeerHandler) HandleTransactionSent(_ *wire.MsgTx, _ p2p.PeerI) error {
	return nil
}

func (bs *PeerHandler) HandleTransactionAnnouncement(_ *wire.InvVect, _ p2p.PeerI) error {
	return nil
}

func (bs *PeerHandler) HandleTransactionRejection(_ *wire.MsgReject, _ p2p.PeerI) error {
	return nil
}

func (bs *PeerHandler) HandleTransaction(msg *wire.MsgTx, peer p2p.PeerI) error {
	return nil
}

func (bs *PeerHandler) HandleBlockAnnouncement(msg *wire.InvVect, peer p2p.PeerI) error {
	pair := utils.NewPair(msg.Hash.CloneBytes(), peer)
	utils.SafeSend(bs.workerCh, pair)

	return nil
}

func (bs *PeerHandler) HandleBlock(msg *wire.MsgBlock, peer p2p.PeerI) error {
	blockHash := msg.BlockHash()
	blockHashBytes := blockHash.CloneBytes()

	txs := make([][]byte, msg.TxCount)

	coinbaseTx := wire.NewMsgTx(1)
	if err := coinbaseTx.Bsvdecode(msg.TransactionReader, msg.ProtocolVersion, msg.MessageEncoding); err != nil {
		return fmt.Errorf("unable to read transaction from block %s: %v", blockHash.String(), err)
	}

	txHash := coinbaseTx.TxHash()
	// coinbase tx is always the first tx in the block
	txs[0] = txHash.CloneBytes()

	height := extractHeightFromCoinbaseTx(coinbaseTx)

	previousBlockHash := msg.Header.PrevBlock
	previousBlockHashBytes := previousBlockHash.CloneBytes()

	merkleRoot := msg.Header.MerkleRoot
	merkleRootBytes := merkleRoot.CloneBytes()

	blockId, err := bs.insertBlock(blockHashBytes, merkleRootBytes, previousBlockHashBytes, height, peer)
	if err != nil {
		return fmt.Errorf("unable to insert block %s: %v", blockHash.String(), err)
	}

	for i := uint64(1); i < msg.TxCount; i++ {
		tx := wire.NewMsgTx(1)
		if err = tx.Bsvdecode(msg.TransactionReader, msg.ProtocolVersion, msg.MessageEncoding); err != nil {
			return fmt.Errorf("unable to read transaction from block %s: %v", blockHash.String(), err)
		}
		txHash = tx.TxHash()
		txs[i] = txHash.CloneBytes()
	}

	if err = bs.markTransactionsAsMined(blockId, txs); err != nil {
		return fmt.Errorf("unable to mark block as mined %s: %v", blockHash.String(), err)
	}

	calculatedMerkleRoot := blockchain.BuildMerkleTreeStore(txs)
	if !bytes.Equal(calculatedMerkleRoot[len(calculatedMerkleRoot)-1], merkleRootBytes) {
		return fmt.Errorf("merkle root mismatch for block %s", blockHash.String())
	}

	block := &p2p.Block{
		Hash:         blockHashBytes,
		MerkleRoot:   merkleRootBytes,
		PreviousHash: previousBlockHashBytes,
		Height:       height,
	}

	if err = bs.markBlockAsProcessed(block); err != nil {
		return fmt.Errorf("unable to mark block as processed %s: %v", blockHash.String(), err)
	}

	return nil
}

func (bs *PeerHandler) insertBlock(blockHash []byte, merkleRoot []byte, previousBlockHash []byte, height uint64, peer p2p.PeerI) (uint64, error) {
	if height >= 1111 { // TODO get the first height we ever processed
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
	err := bs.store.MarkBlockAsDone(context.Background(), block.Hash)
	if err != nil {
		return err
	}

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
