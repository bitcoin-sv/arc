package merkle_verifier

import (
	"context"
	"errors"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/ccoveille/go-safecast"

	"github.com/bitcoin-sv/arc/internal/blocktx"
)

var (
	ErrVerifyMerkleRoots = errors.New("failed to verify merkle roots")
)

type MerkleVerifier struct {
	verifier blocktx.MerkleRootsVerifier
}

func New(v blocktx.MerkleRootsVerifier) MerkleVerifier {
	return MerkleVerifier{verifier: v}
}

func (a MerkleVerifier) IsValidRootForHeight(root *chainhash.Hash, height uint32) (bool, error) {
	ctx := context.Background()

	heightUint64, err := safecast.ToUint64(height)
	if err != nil {
		return false, err
	}

	blocktxReq := []blocktx.MerkleRootVerificationRequest{{MerkleRoot: root.String(), BlockHeight: heightUint64}}

	unverifiedBlockHeights, err := a.verifier.VerifyMerkleRoots(ctx, blocktxReq)
	if err != nil {
		return false, errors.Join(ErrVerifyMerkleRoots, err)
	}

	if len(unverifiedBlockHeights) == 0 {
		return true, nil
	}

	return false, nil
}
