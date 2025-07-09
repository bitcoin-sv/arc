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
	blocktx  blocktx.Client
}

func New(v blocktx.MerkleRootsVerifier, blocktx blocktx.Client) MerkleVerifier {
	return MerkleVerifier{verifier: v, blocktx: blocktx}
}

func (a MerkleVerifier) IsValidRootForHeight(ctx context.Context, root *chainhash.Hash, height uint32) (bool, error) {
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

func (a MerkleVerifier) CurrentHeight(ctx context.Context) (uint32, error) {
	height, err := a.blocktx.CurrentBlockHeight(ctx)
	if err != nil {
		return 0, err
	}

	return uint32(height.CurrentBlockHeight), nil // #nosec G115
}
