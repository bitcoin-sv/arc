package store

import (
	"encoding/hex"
	"slices"

	"github.com/libsv/go-p2p/chaincfg/chainhash"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
)

type BlockGap struct {
	Height uint64
	Hash   *chainhash.Hash
}

type TxHashWithMerkleTreeIndex struct {
	Hash            []byte
	MerkleTreeIndex int64
}

type BlockTransactionWithMerklePath struct {
	BlockTransaction
	MerklePath string
}

type BlockTransaction struct {
	TxHash          []byte
	BlockHash       []byte
	BlockHeight     uint64
	MerkleTreeIndex int64
	BlockStatus     blocktx_api.Status
	MerkleRoot      []byte
}

type BlockStatusUpdate struct {
	Hash   []byte
	Status blocktx_api.Status
}

func (b *BlockTransaction) GetMerkleRootString() string {
	txMerkleRoot := make([]byte, len(b.MerkleRoot))
	copy(txMerkleRoot, b.MerkleRoot)
	slices.Reverse(txMerkleRoot)
	merkleRoot := hex.EncodeToString(txMerkleRoot)

	return merkleRoot
}
