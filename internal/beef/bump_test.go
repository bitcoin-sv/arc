package beef

import (
	"errors"
	"testing"

	testutils "github.com/bitcoin-sv/arc/pkg/test_utils"
	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculateMerkleRootsFromBumps(t *testing.T) {
	testCases := []struct {
		name                string
		bumpStr             string
		expectedMerkleRoots []MerkleRootVerificationRequest
		expectedError       error
	}{
		{
			name:    "success",
			bumpStr: "fe636d0c0007021400fe507c0c7aa754cef1f7889d5fd395cf1f785dd7de98eed895dbedfe4e5bc70d1502ac4e164f5bc16746bb0868404292ac8318bbac3800e4aad13a014da427adce3e010b00bc4ff395efd11719b277694cface5aa50d085a0bb81f613f70313acd28cf4557010400574b2d9142b8d28b61d88e3b2c3f44d858411356b49a28a4643b6d1a6a092a5201030051a05fc84d531b5d250c23f4f886f6812f9fe3f402d61607f977b4ecd2701c19010000fd781529d58fc2523cf396a7f25440b409857e7e221766c57214b1d38c7b481f01010062f542f45ea3660f86c013ced80534cb5fd4c19d66c56e7e8c5d4bf2d40acc5e010100b121e91836fd7cd5102b654e9f72f3cf6fdbfd0b161c53a9c54b12c841126331",
			expectedMerkleRoots: []MerkleRootVerificationRequest{{
				MerkleRoot:  "bb6f640cc4ee56bf38eb5a1969ac0c16caa2d3d202b22bf3735d10eec0ca6e00",
				BlockHeight: 814435,
			}},
			expectedError: nil,
		},
		{
			name:                "no transactions marked as expected in bump",
			bumpStr:             "fe636d0c0007021400fe507c0c7aa754cef1f7889d5fd395cf1f785dd7de98eed895dbedfe4e5bc70d1500ac4e164f5bc16746bb0868404292ac8318bbac3800e4aad13a014da427adce3e010b00bc4ff395efd11719b277694cface5aa50d085a0bb81f613f70313acd28cf4557010400574b2d9142b8d28b61d88e3b2c3f44d858411356b49a28a4643b6d1a6a092a5201030051a05fc84d531b5d250c23f4f886f6812f9fe3f402d61607f977b4ecd2701c19010000fd781529d58fc2523cf396a7f25440b409857e7e221766c57214b1d38c7b481f01010062f542f45ea3660f86c013ced80534cb5fd4c19d66c56e7e8c5d4bf2d40acc5e010100b121e91836fd7cd5102b654e9f72f3cf6fdbfd0b161c53a9c54b12c841126331",
			expectedMerkleRoots: nil,
			expectedError:       errors.New("no transactions marked as expected to verify in bump"),
		},
		{
			name:                "different merkle roots for the same block",
			bumpStr:             "fe8a6a0c000c04fde80b0011774f01d26412f0d16ea3f0447be0b5ebec67b0782e321a7a01cbdf7f734e30fde90b02004e53753e3fe4667073063a17987292cfdea278824e9888e52180581d7188d8fdea0b025e442996fc53f0191d649e68a200e752fb5f39e0d5617083408fa179ddc5c998fdeb0b0102fdf405000671394f72237d08a4277f4435e5b6edf7adc272f25effef27cdfe805ce71a81fdf50500262bccabec6c4af3ed00cc7a7414edea9c5efa92fb8623dd6160a001450a528201fdfb020101fd7c010093b3efca9b77ddec914f8effac691ecb54e2c81d0ab81cbc4c4b93befe418e8501bf01015e005881826eb6973c54003a02118fe270f03d46d02681c8bc71cd44c613e86302f8012e00e07a2bb8bb75e5accff266022e1e5e6e7b4d6d943a04faadcf2ab4a22f796ff30116008120cafa17309c0bb0e0ffce835286b3a2dcae48e4497ae2d2b7ced4f051507d010a00502e59ac92f46543c23006bff855d96f5e648043f0fb87a7a5949e6a9bebae430104001ccd9f8f64f4d0489b30cc815351cf425e0e78ad79a589350e4341ac165dbe45010301010000af8764ce7e1cc132ab5ed2229a005c87201c9a5ee15c0f91dd53eff31ab30cd4",
			expectedMerkleRoots: nil,
			expectedError:       errors.New("different merkle roots for the same block"),
		},
	}

	for _, tc := range testCases {
		testutils.RunParallel(t, true, tc.name, func(t *testing.T) {
			// given
			bump, err := sdkTx.NewMerklePathFromHex(tc.bumpStr)
			require.NoError(t, err)

			// when
			actualMerkleRoots, err := CalculateMerkleRootsFromBumps([]*sdkTx.MerklePath{bump})

			// then
			assert.Equal(t, tc.expectedMerkleRoots, actualMerkleRoots)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
