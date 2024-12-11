package merkle_verifier

import (
	"context"

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
