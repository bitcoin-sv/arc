package blocktx

import (
	"context"
	"database/sql"
	"log"

	"github.com/TAAL-GmbH/arc/blocktx/blocktx_api"
	"github.com/TAAL-GmbH/arc/blocktx/store"
	"github.com/TAAL-GmbH/arc/p2p"
	"github.com/TAAL-GmbH/arc/p2p/chaincfg/chainhash"
	"github.com/TAAL-GmbH/arc/p2p/wire"
	"github.com/ordishs/go-utils"
)

type BlockTxPeerStore struct {
	workerCh chan utils.Pair[[]byte, p2p.PeerI]
	store    store.Interface
}

func NewBlockTxPeerStore(store store.Interface) p2p.PeerStoreI {
	s := &BlockTxPeerStore{
		store:    store,
		workerCh: make(chan utils.Pair[[]byte, p2p.PeerI], 100),
	}

	go func() {
		for pair := range s.workerCh {
			hash, err := chainhash.NewHash(pair.First)
			if err != nil {
				log.Printf("ERROR: ProcessBlock: %s", err)
				continue
			}

			if _, err := store.GetBlock(context.Background(), hash.CloneBytes()); err != nil {
				if err == sql.ErrNoRows {
					store.InsertBlock(context.Background(), &blocktx_api.Block{
						Hash: hash.CloneBytes(),
					})

					peer := pair.Second

					msg := wire.NewMsgGetData()
					if err := msg.AddInvVect(wire.NewInvVect(wire.InvTypeBlock, hash)); err != nil {
						log.Printf("ERROR: ProcessBlock: %s", err)
						continue
					}

					peer.WriteMsg(msg)
					log.Printf("INFO: ProcessBlock: %s", hash.String())
				} else {
					log.Printf("ERROR: ProcessBlock: %s", err)
				}
			}
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
