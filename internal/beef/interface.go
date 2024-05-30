package beef

import "context"

type MerkleRootsVerificator interface {
	VerifyMerkleRoots(ctx context.Context, merkleRootVerificationRequest []MerkleRootVerificationRequest) ([]uint64, error)
}

type MerkleRootVerificationRequest struct {
	MerkleRoot  string
	BlockHeight uint64
}
