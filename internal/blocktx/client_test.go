package blocktx_test

import (
	"context"
	"errors"
	"testing"

	testutils "github.com/bitcoin-sv/arc/pkg/test_utils"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/mocks"
)

func TestClient_VerifyMerkleRoots(t *testing.T) {
	tt := []struct {
		name      string
		verifyErr error

		expectedErrorStr string
	}{
		{
			name:      "success",
			verifyErr: nil,
		},
		{
			name:             "err",
			verifyErr:        errors.New("failed to verify merkle roots"),
			expectedErrorStr: "failed to verify merkle roots",
		},
	}

	for _, tc := range tt {
		testutils.RunParallel(t, true, tc.name, func(t *testing.T) {
			apiClient := &mocks.BlockTxAPIClientMock{
				VerifyMerkleRootsFunc: func(_ context.Context, _ *blocktx_api.MerkleRootsVerificationRequest, _ ...grpc.CallOption) (*blocktx_api.MerkleRootVerificationResponse, error) {
					return &blocktx_api.MerkleRootVerificationResponse{
						UnverifiedBlockHeights: []uint64{81190, 89022},
					}, tc.verifyErr
				},
			}
			client := blocktx.NewClient(apiClient)

			merkleRootsRequest := []blocktx.MerkleRootVerificationRequest{
				{MerkleRoot: "merkle_root", BlockHeight: 81190},
				{MerkleRoot: "merkle_root_2", BlockHeight: 89022},
			}

			res, err := client.VerifyMerkleRoots(context.Background(), merkleRootsRequest)
			if tc.expectedErrorStr != "" {
				require.ErrorContains(t, err, tc.expectedErrorStr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, []uint64{81190, 89022}, res)
		})
	}
}

func TestClient_RegisterTransaction(t *testing.T) {
	tt := []struct {
		name        string
		registerErr error

		expectedError error
	}{
		{
			name:        "success",
			registerErr: nil,
		},
		{
			name:          "err",
			registerErr:   errors.New("failed to register"),
			expectedError: blocktx.ErrRegisterTransaction,
		},
	}

	for _, tc := range tt {
		testutils.RunParallel(t, true, tc.name, func(t *testing.T) {
			apiClient := &mocks.BlockTxAPIClientMock{
				RegisterTransactionFunc: func(_ context.Context, _ *blocktx_api.Transaction, _ ...grpc.CallOption) (*emptypb.Empty, error) {
					return &emptypb.Empty{}, tc.registerErr
				},
			}
			client := blocktx.NewClient(apiClient)

			err := client.RegisterTransaction(context.Background(), []byte("hash"))
			if tc.expectedError != nil {
				require.ErrorIs(t, err, tc.expectedError)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestClient_RegisterTransactions(t *testing.T) {
	tt := []struct {
		name        string
		registerErr error

		expectedError error
	}{
		{
			name:        "success",
			registerErr: nil,
		},
		{
			name:          "err",
			registerErr:   errors.New("failed to register"),
			expectedError: blocktx.ErrRegisterTransaction,
		},
	}

	for _, tc := range tt {
		testutils.RunParallel(t, true, tc.name, func(t *testing.T) {
			apiClient := &mocks.BlockTxAPIClientMock{
				RegisterTransactionsFunc: func(_ context.Context, _ *blocktx_api.Transactions, _ ...grpc.CallOption) (*emptypb.Empty, error) {
					return &emptypb.Empty{}, tc.registerErr
				},
			}
			client := blocktx.NewClient(apiClient)

			err := client.RegisterTransactions(context.Background(), [][]byte{[]byte("hash")})
			if tc.expectedError != nil {
				require.ErrorIs(t, err, tc.expectedError)
				return
			}

			require.NoError(t, err)
		})
	}
}
