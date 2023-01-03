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

func (m *MetamorphPeerStore) ProcessBlock(hash []byte) error {
	return nil
}
