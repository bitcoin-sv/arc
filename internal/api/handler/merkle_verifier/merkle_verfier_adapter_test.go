package merkle_verifier

import (
	"context"
	"errors"
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/api/handler/merkle_verifier/mocks"
	"github.com/bitcoin-sv/arc/internal/blocktx"
)

func TestMerkleVerifier_IsValidRootForHeight(t *testing.T) {
	tt := []struct {
		name                   string
		unverifiedBlockHeights []uint64
		verifyErr              error

		expectedOk    bool
		expectedError error
	}{
		{
			name:                   "valid root for height",
			expectedOk:             true,
			unverifiedBlockHeights: []uint64{},
		},
		{
			name:      "error",
			verifyErr: errors.New("some error"),

			expectedError: ErrVerifyMerkleRoots,
			expectedOk:    false,
		},
		{
			name:                   "invalid root for height",
			expectedOk:             false,
			unverifiedBlockHeights: []uint64{10},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			rootsVerifier := &mocks.MerkleRootsVerifierMock{
				VerifyMerkleRootsFunc: func(_ context.Context, _ []blocktx.MerkleRootVerificationRequest) ([]uint64, error) {
					return tc.unverifiedBlockHeights, tc.verifyErr
				},
			}

			sut := New(rootsVerifier)

			root, err := chainhash.NewHashFromHex("c0603858c68bc1445eb8cefce71c556d511b1b9a82a3de138dd3470dd1422676")
			require.NoError(t, err)

			actualOk, err := sut.IsValidRootForHeight(root, 5)

			require.Equal(t, tc.expectedOk, actualOk)
			if tc.expectedError != nil {
				require.ErrorIs(t, err, tc.expectedError)
				return
			}

			require.NoError(t, err)
		})
	}
}
