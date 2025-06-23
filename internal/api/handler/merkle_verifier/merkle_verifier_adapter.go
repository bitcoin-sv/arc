package merkle_verifier

import (
	"context"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/ccoveille/go-safecast"

	"github.com/bitcoin-sv/arc/internal/beef"
	"github.com/bitcoin-sv/arc/internal/blocktx"
)

type MerkleVerifier struct {
	v blocktx.MerkleRootsVerifier
}

func New(v blocktx.MerkleRootsVerifier) MerkleVerifier {
	return MerkleVerifier{v: v}
}

func (a MerkleVerifier) Verify(ctx context.Context, request []beef.MerkleRootVerificationRequest) ([]uint64, error) {
	blocktxReq := mapToBlocktxMerkleVerRequest(request)
	return a.v.VerifyMerkleRoots(ctx, blocktxReq)
}

func (a MerkleVerifier) IsValidRootForHeight(root *chainhash.Hash, height uint32) (bool, error) {
	ctx := context.Background()

	heightUint64, err := safecast.ToUint64(height)
	if err != nil {
		return false, err
	}

	blocktxReq := []blocktx.MerkleRootVerificationRequest{{MerkleRoot: root.String(), BlockHeight: heightUint64}}

	unverifiedBlockHeights, err := a.v.VerifyMerkleRoots(ctx, blocktxReq)
	if err != nil {
		return false, err
	}

	if len(unverifiedBlockHeights) == 0 {
		return true, nil
	}

	return false, nil
}

func mapToBlocktxMerkleVerRequest(mrReq []beef.MerkleRootVerificationRequest) []blocktx.MerkleRootVerificationRequest {
	merkleRoots := make([]blocktx.MerkleRootVerificationRequest, 0, len(mrReq))

	for _, mr := range mrReq {
		merkleRoots = append(merkleRoots, blocktx.MerkleRootVerificationRequest{
			BlockHeight: mr.BlockHeight,
			MerkleRoot:  mr.MerkleRoot,
		})
	}

	return merkleRoots
}
