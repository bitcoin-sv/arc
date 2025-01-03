package store

import (
	"github.com/libsv/go-p2p/chaincfg/chainhash"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
)

type BlockGap struct {
	Height uint64
	Hash   *chainhash.Hash
}

type TxWithMerklePath struct { // Todo: Rename TxWithMerkleTreeIndex
	Hash            []byte
	MerkleTreeIndex int64
}

type TransactionBlock struct { //Todo: Embed BlockTransaction & rename
	TxHash          []byte
	BlockHash       []byte
	BlockHeight     uint64
	MerklePath      string
	MerkleTreeIndex int64
	BlockStatus     blocktx_api.Status
}

type BlockTransaction struct {
	TxHash          []byte
	BlockHash       []byte
	BlockHeight     uint64
	MerkleTreeIndex int64
	BlockStatus     blocktx_api.Status
}

type BlockStatusUpdate struct {
	Hash   []byte
	Status blocktx_api.Status
}
