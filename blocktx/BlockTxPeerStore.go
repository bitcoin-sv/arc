package blocktx

import (
	"context"
	"sync"
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
	announcedMu    sync.Mutex
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

			// if _, err := store.GetBlock(context.Background(), hash.CloneBytes()); err != nil {
			// 	if err == sql.ErrNoRows {
			// store.InsertBlock(context.Background(), &blocktx_api.Block{
			// 	Hash: hash.CloneBytes(),
			// })

			peer := pair.Second

			msg := wire.NewMsgGetData()
			if err := msg.AddInvVect(wire.NewInvVect(wire.InvTypeBlock, hash)); err != nil {
				logger.Errorf("ProcessBlock: %s", err)
				continue
			}

			peer.WriteMsg(msg)
			logger.Infof("ProcessBlock: %s", hash.String())
			// } else {
			// 	logger.Errorf("ProcessBlock: %s", err)
			// }
			// }
		}
	}()

	return s
}

func (m *BlockTxPeerStore) GetTransactionBytes(txID []byte) ([]byte, error) {
	return nil, nil
}

func (m *BlockTxPeerStore) HandleBlockAnnouncement(hash []byte, peer p2p.PeerI) error {
	m.announcedMu.Lock()
	defer m.announcedMu.Unlock()

	id := utils.HexEncodeAndReverseBytes(hash)
	item, found := m.announcedCache.Get(id)
	if !found {
		m.announcedCache.Set(id, []p2p.PeerI{peer})
		pair := utils.NewPair(hash, peer)
		utils.SafeSend(m.workerCh, pair)

	} else {
		item = append(item, peer)
		m.announcedCache.Set(id, item)
	}

	return nil
}

func (m *BlockTxPeerStore) InsertBlock(blockHash []byte, blockHeader []byte, height uint64) (uint64, error) {
	return m.store.InsertBlock(context.Background(), &blocktx_api.Block{
		Hash:   blockHash,
		Header: blockHeader,
		Height: height,
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
