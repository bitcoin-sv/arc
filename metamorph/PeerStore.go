package metamorph

import (
	"context"

	"github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/TAAL-GmbH/arc/p2p"
)

type PeerStore struct {
	store store.Store
}

func NewPeerStore(s store.Store) p2p.PeerStoreI {
	return &PeerStore{
		store: s,
	}
}

func (m *PeerStore) GetTransactionBytes(txID []byte) ([]byte, error) {
	sd, err := m.store.Get(context.Background(), txID)
	if err != nil {
		return nil, err
	}
	return sd.RawTx, nil
}

func (m *PeerStore) HandleBlockAnnouncement(hash []byte, peer p2p.PeerI) error {
	return nil
}

func (m *PeerStore) InsertBlock(blockHash []byte, merkleRoot []byte, prevhash []byte, height uint64, peer p2p.PeerI) (uint64, error) {
	return 0, nil
}

func (m *PeerStore) MarkTransactionsAsMined(blockId uint64, txHashes [][]byte) error {
	return nil
}

func (m *PeerStore) MarkBlockAsProcessed(block *p2p.Block) error {
	return nil
}
