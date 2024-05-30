package merklerootsverificator

import (
	"context"

	"github.com/bitcoin-sv/arc/pkg/blocktx"
)

type allowAllMerkleRootsVerificator struct{}

func (c *allowAllMerkleRootsVerificator) VerifyMerkleRoots(ctx context.Context, merkleRootVerificationRequest []blocktx.MerkleRootVerificationRequest) ([]uint64, error) {
	// Verify all BUMPs as correct
	return nil, nil
}

// Returns a MerkleRootsVerificator that accepts all merkle roots.
// For test purposes only!
func NewAllowAllVerificator() blocktx.MerkleRootsVerificator {
	return &allowAllMerkleRootsVerificator{}
}
