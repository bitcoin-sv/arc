package blocktx

import (
	"github.com/TAAL-GmbH/arc/p2p"
)

type BlockTxPeerStore struct{}

func NewBlockTxPeerStore() p2p.PeerStoreI {
	return &BlockTxPeerStore{}
}

func (m *BlockTxPeerStore) GetTransactionBytes(txID []byte) ([]byte, error) {
	return nil, nil
}

func (m *BlockTxPeerStore) ProcessBlock(hash []byte) error {
	return nil
}
