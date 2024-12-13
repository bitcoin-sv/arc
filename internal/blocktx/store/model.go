package store

import (
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

type BlockGap struct {
	Height uint64
	Hash   *chainhash.Hash
}

type TxWithMerklePath struct {
	Hash            []byte
	MerklePath      string
	MerkleTreeIndex int64
}

type TransactionBlock struct {
	TxHash          []byte
	BlockHash       []byte
	BlockHeight     uint64
	MerklePath      string
	MerkleTreeIndex int64
	BlockStatus     blocktx_api.Status
}

type BlockStatusUpdate struct {
	Hash   []byte
	Status blocktx_api.Status
}
