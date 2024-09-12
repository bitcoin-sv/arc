package beef

import (
	"errors"

	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
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
	blockMerkleRoot := ""

	for _, pathElement := range bump.Path {
		for _, pe := range pathElement {
			if pe.Txid != nil {
				txId := pe.Hash.String()
				mr, err := bump.ComputeRoot(&txId)
				if err != nil {
					return "", err
				}
				if blockMerkleRoot == "" {
					blockMerkleRoot = mr
				} else if blockMerkleRoot != mr {
					return "", ErrBUMPDifferentMerkleRoots
				}
			}
		}
	}

	if blockMerkleRoot == "" {
		return "", ErrBUMPEmptyMerkleRoot
	}
	return blockMerkleRoot, nil
}
