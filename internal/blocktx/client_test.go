package blocktx_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bitcoin-sv/arc/internal/blocktx"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/mocks"
)

func TestClient_DelUnfinishedBlockProcessing(t *testing.T) {
	tt := []struct {
		name   string
		delErr error

		expectedErrorStr string
	}{
		{
			name: "success",
		},
		{
			name:   "err",
			delErr: errors.New("failed to delete unfinished block processing"),

			expectedErrorStr: "failed to delete unfinished block processing",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			apiClient := &mocks.BlockTxAPIClientMock{
				DelUnfinishedBlockProcessingFunc: func(_ context.Context, _ *blocktx_api.DelUnfinishedBlockProcessingRequest, _ ...grpc.CallOption) (*blocktx_api.RowsAffectedResponse, error) {
					return &blocktx_api.RowsAffectedResponse{}, tc.delErr
				},
			}
			client := blocktx.NewClient(apiClient)

			_, err := client.DelUnfinishedBlockProcessing(context.Background(), "test-1")
			if tc.expectedErrorStr != "" {
				require.ErrorContains(t, err, tc.expectedErrorStr)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestClient_ClearBlocks(t *testing.T) {
	tt := []struct {
		name     string
		clearErr error

		expectedErrorStr string
	}{
		{
			name: "success",
		},
		{
			name:     "err",
			clearErr: errors.New("failed to clear data"),

			expectedErrorStr: "failed to clear data",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			apiClient := &mocks.BlockTxAPIClientMock{
				ClearBlocksFunc: func(_ context.Context, _ *blocktx_api.ClearData, _ ...grpc.CallOption) (*blocktx_api.RowsAffectedResponse, error) {
					return &blocktx_api.RowsAffectedResponse{Rows: 5}, tc.clearErr
				},
			}
			client := blocktx.NewClient(apiClient)

			res, err := client.ClearBlocks(context.Background(), 1)
			if tc.expectedErrorStr != "" {
				require.ErrorContains(t, err, tc.expectedErrorStr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, int64(5), res)
		})
	}
}

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
		t.Run(tc.name, func(t *testing.T) {
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
