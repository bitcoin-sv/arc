package merklerootsverifier

import (
	"context"

	"github.com/bitcoin-sv/arc/pkg/blocktx"
)

type allowAllMerkleRootsVerifier struct{}

func (c *allowAllMerkleRootsVerifier) VerifyMerkleRoots(ctx context.Context, merkleRootVerificationRequest []blocktx.MerkleRootVerificationRequest) ([]uint64, error) {
	// Verify all BUMPs as correct
	return nil, nil
}

// Returns a MerkleRootsVerifier that accepts all merkle roots.
// For test purposes only!
func NewAllowAllVerifier() blocktx.MerkleRootsVerifier {
	return &allowAllMerkleRootsVerifier{}
}
