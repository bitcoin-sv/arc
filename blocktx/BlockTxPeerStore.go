package blocktx

import (
	"log"

	"github.com/TAAL-GmbH/arc/p2p"
	"github.com/ordishs/go-utils"
)

type BlockTxPeerStore struct{}

func NewBlockTxPeerStore() p2p.PeerStoreI {
	return &BlockTxPeerStore{}
}

func (m *BlockTxPeerStore) GetTransactionBytes(txID []byte) ([]byte, error) {
	return nil, nil
}

func (m *BlockTxPeerStore) ProcessBlock(hash []byte) error {
	log.Printf("ProcessBlock: %s", utils.HexEncodeAndReverseBytes(hash))
	return nil
}
