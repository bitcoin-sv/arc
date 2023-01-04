package metamorph

import (
	"context"

	"github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/TAAL-GmbH/arc/p2p"
)

type MetamorphPeerStore struct {
	store store.Store
}

func NewMetamorphPeerStore(s store.Store) p2p.PeerStoreI {
	return &MetamorphPeerStore{
		store: s,
	}
}

func (m *MetamorphPeerStore) GetTransactionBytes(txID []byte) ([]byte, error) {
	sd, err := m.store.Get(context.Background(), txID)
	if err != nil {
		return nil, err
	}
	return sd.RawTx, nil
}

func (m *MetamorphPeerStore) HandleBlockAnnouncement(hash []byte, peer p2p.PeerI) error {
	return nil
}

func (m *MetamorphPeerStore) InsertBlock(blockHash []byte, blockHeader []byte, height uint64) (uint64, error) {
	return 0, nil
}

func (m *MetamorphPeerStore) MarkTransactionsAsMined(blockId uint64, txHashes [][]byte) error {
	return nil
}

func (m *MetamorphPeerStore) MarkBlockAsProcessed(blockId uint64) error {
	return nil
}
