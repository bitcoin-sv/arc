package merklerootsverifier

import (
	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/blocktx/mocks"
)

// NewAllowAllVerifier Returns a MerkleRootsVerifier that accepts all merkle roots.
// For test purposes only!
func NewAllowAllVerifier() blocktx.Client {
	return &mocks.ClientMock{}
}
