package beef

import (
	"errors"

	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
)

var (
	ErrBUMPNoMerkleRoots        = errors.New("no merkle roots found for validation")
	ErrBUMPDifferentMerkleRoots = errors.New("different merkle roots for the same block")
	ErrBUMPEmptyMerkleRoot      = errors.New("no transactions marked as expected to verify in bump")
)

type MerkleRootVerificationRequest struct {
	MerkleRoot  string
	BlockHeight uint64
}

func CalculateMerkleRootsFromBumps(bumps []*sdkTx.MerklePath) ([]MerkleRootVerificationRequest, error) {
	merkleRoots := make([]MerkleRootVerificationRequest, 0)

	for _, bump := range bumps {
		blockMerkleRoot, err := calculateMerkleRootFromBump(bump)
		if err != nil {
			return nil, err
		}

		merkleRoots = append(merkleRoots, MerkleRootVerificationRequest{
			MerkleRoot:  blockMerkleRoot,
			BlockHeight: uint64(bump.BlockHeight),
		})
	}

	if len(merkleRoots) == 0 {
		return nil, ErrBUMPNoMerkleRoots
	}

	return merkleRoots, nil
}

func calculateMerkleRootFromBump(bump *sdkTx.MerklePath) (string, error) {
	var computedRoot string

	for _, pathGroup := range bump.Path {
		for _, element := range pathGroup {
			if element.Txid == nil {
				continue
			}

			root, err := bump.ComputeRoot(element.Hash)
			if err != nil {
				return "", err
			}

			rootStr := root.String()
			if computedRoot == "" {
				computedRoot = rootStr
			} else if computedRoot != rootStr {
				return "", ErrBUMPDifferentMerkleRoots
			}
		}
	}

	if computedRoot == "" {
		return "", ErrBUMPEmptyMerkleRoot
	}

	return computedRoot, nil
}
