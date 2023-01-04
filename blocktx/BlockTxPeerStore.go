package blocktx

import (
	"context"
	"database/sql"
	"time"

	"github.com/TAAL-GmbH/arc/blocktx/blocktx_api"
	"github.com/TAAL-GmbH/arc/blocktx/store"
	"github.com/TAAL-GmbH/arc/p2p"
	"github.com/TAAL-GmbH/arc/p2p/chaincfg/chainhash"
	"github.com/TAAL-GmbH/arc/p2p/wire"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/go-utils/expiringmap"
)

type BlockTxPeerStore struct {
	workerCh       chan utils.Pair[[]byte, p2p.PeerI]
	store          store.Interface
	logger         utils.Logger
	announcedCache *expiringmap.ExpiringMap[string, []p2p.PeerI]
}

func NewBlockTxPeerStore(store store.Interface, logger utils.Logger) p2p.PeerStoreI {
	s := &BlockTxPeerStore{
		store:          store,
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

			peer.WriteMsg(msg)
			logger.Infof("ProcessBlock: %s", hash.String())
		}
	}()

	return s
}

func (m *BlockTxPeerStore) GetTransactionBytes(txID []byte) ([]byte, error) {
	return nil, nil
}

func (m *BlockTxPeerStore) HandleBlockAnnouncement(hash []byte, peer p2p.PeerI) error {
	pair := utils.NewPair(hash, peer)
	utils.SafeSend(m.workerCh, pair)

	return nil
}

func (m *BlockTxPeerStore) InsertBlock(blockHash []byte, merkleRoot []byte, previousBlockHash []byte, height uint64, peer p2p.PeerI) (uint64, error) {
	if height >= 1111 { // TODO get the first height we ever processed
		if _, found := m.announcedCache.Get(utils.HexEncodeAndReverseBytes(previousBlockHash)); !found {
			if _, err := m.store.GetBlock(context.Background(), previousBlockHash); err != nil {
				if err == sql.ErrNoRows {
					pair := utils.NewPair(previousBlockHash, peer)
					utils.SafeSend(m.workerCh, pair)
				}
			}
		}
	}

	return m.store.InsertBlock(context.Background(), &blocktx_api.Block{
		Hash:       blockHash,
		Merkleroot: merkleRoot,
		Prevhash:   previousBlockHash,
		Height:     height,
	})
}

func (m *BlockTxPeerStore) MarkTransactionsAsMined(blockId uint64, transactions [][]byte) error {
	txs := make([]*blocktx_api.Transaction, 0, len(transactions))

	for _, tx := range transactions {
		txs = append(txs, &blocktx_api.Transaction{
			Hash: tx,
		})
	}

	if err := m.store.InsertBlockTransactions(context.Background(), blockId, txs); err != nil {
		return err
	}

	return nil
}

func (m *BlockTxPeerStore) MarkBlockAsProcessed(blockId uint64) error {
	return m.store.MarkBlockAsDone(context.Background(), blockId)
}
