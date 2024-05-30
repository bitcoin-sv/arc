package beef

import (
	"errors"

	"github.com/libsv/go-bc"
)

func CalculateMerkleRootsFromBumps(bumps []*bc.BUMP) ([]MerkleRootVerificationRequest, error) {
	merkleRoots := make([]MerkleRootVerificationRequest, 0)

	for _, bump := range bumps {
		blockMerkleRoot := ""

		for _, txId := range bump.Txids() {
			mr, err := bump.CalculateRootGivenTxid(txId)
			if err != nil {
				return nil, err
			}
			if blockMerkleRoot == "" {
				blockMerkleRoot = mr
			} else if blockMerkleRoot != mr {
				return nil, errors.New("different merkle roots for the same block")
			}
		}

		if blockMerkleRoot == "" {
			return nil, errors.New("no transactions marked as expected to verify in bump")
		}

		merkleRoots = append(merkleRoots, MerkleRootVerificationRequest{
			MerkleRoot:  blockMerkleRoot,
			BlockHeight: bump.BlockHeight,
		})
	}

	if len(merkleRoots) == 0 {
		return nil, errors.New("no merkle roots found for validation")
	}

	return merkleRoots, nil
}
