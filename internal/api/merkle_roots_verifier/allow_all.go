package merklerootsverifier

import (
	"github.com/bitcoin-sv/arc/internal/global"
	"github.com/bitcoin-sv/arc/internal/global/mocks"
)

// NewAllowAllVerifier Returns a MerkleRootsVerifier that accepts all merkle roots.
// For test purposes only!
func NewAllowAllVerifier() global.Client {
	return &mocks.ClientMock{}
}
