package beef

import (
	"errors"

	"github.com/libsv/go-bc"
)

// BUMPs represents a slice of BUMPs - BSV Unified Merkle Paths
type BUMPs []*BUMP

// BUMP is a struct that represents a whole BUMP format
type BUMP struct {
	BlockHeight uint64
	Path        [][]BUMPLeaf
}

// BUMPLeaf represents each BUMP path element
type BUMPLeaf struct {
	Hash      string
	TxId      bool
	Duplicate bool
	Offset    uint64
}

// Flags which are used to determine the type of BUMPLeaf
const (
	dataFlag byte = iota
	duplicateFlag
	txIDFlag
)

func (b BUMP) CalculateMerkleRoot() (string, error) {
	merkleRoot := ""

	for _, bumpPathElement := range b.Path[0] {
		if bumpPathElement.TxId {
			calcMerkleRoot, err := calculateMerkleRoot(bumpPathElement, b)
			if err != nil {
				return "", err
			}

			if merkleRoot == "" {
				merkleRoot = calcMerkleRoot
				continue
			}

			if calcMerkleRoot != merkleRoot {
				return "", errors.New("different merkle roots for the same block")
			}
		}
	}
	return merkleRoot, nil
}

// calculateMerkleRoots will calculate one merkle root for tx in the BUMPLeaf
func calculateMerkleRoot(baseLeaf BUMPLeaf, bump BUMP) (string, error) {
	calculatedHash := baseLeaf.Hash
	offset := baseLeaf.Offset

	for _, bLevel := range bump.Path {
		newOffset := getOffsetPair(offset)
		leafInPair := findLeafByOffset(newOffset, bLevel)
		if leafInPair == nil {
			return "", errors.New("could not find pair")
		}

		leftNode, rightNode := prepareNodes(baseLeaf, offset, *leafInPair, newOffset)

		str, err := bc.MerkleTreeParentStr(leftNode, rightNode)
		if err != nil {
			return "", err
		}
		calculatedHash = str

		offset = offset / 2

		baseLeaf = BUMPLeaf{
			Hash:   calculatedHash,
			Offset: offset,
		}
	}

	return calculatedHash, nil
}

func getOffsetPair(offset uint64) uint64 {
	if offset%2 == 0 {
		return offset + 1
	}
	return offset - 1
}

func findLeafByOffset(offset uint64, bumpLeaves []BUMPLeaf) *BUMPLeaf {
	for _, bumpTx := range bumpLeaves {
		if bumpTx.Offset == offset {
			return &bumpTx
		}
	}
	return nil
}

func prepareNodes(baseLeaf BUMPLeaf, offset uint64, leafInPair BUMPLeaf, newOffset uint64) (string, string) {
	var baseLeafHash, pairLeafHash string

	if baseLeaf.Duplicate {
		baseLeafHash = leafInPair.Hash
	} else {
		baseLeafHash = baseLeaf.Hash
	}

	if leafInPair.Duplicate {
		pairLeafHash = baseLeaf.Hash
	} else {
		pairLeafHash = leafInPair.Hash
	}

	if newOffset > offset {
		return baseLeafHash, pairLeafHash
	}
	return pairLeafHash, baseLeafHash
}
