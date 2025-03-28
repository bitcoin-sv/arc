package merklerootsverifier

import (
	"context"

	"github.com/bitcoin-sv/arc/internal/blocktx"
)

type allowAllMerkleRootsVerifier struct{}

func (c *allowAllMerkleRootsVerifier) VerifyMerkleRoots(_ context.Context, _ []blocktx.MerkleRootVerificationRequest) ([]uint64, error) {
	// Verify all BUMPs as correct
	return nil, nil
}

// NewAllowAllVerifier Returns a MerkleRootsVerifier that accepts all merkle roots.
// For test purposes only!
func NewAllowAllVerifier() blocktx.MerkleRootsVerifier {
	return &allowAllMerkleRootsVerifier{}
}
