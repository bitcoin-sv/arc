package metamorph_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/mocks"
	"google.golang.org/grpc"

	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/stretchr/testify/require"
)

//go:generate moq -pkg mocks -out ./mocks/metamorph_api_mock.go ./metamorph_api MetaMorphAPIClient

func TestClient_SetUnlockedByName(t *testing.T) {
	tt := []struct {
		name           string
		setUnlockedErr error

		expectedErrorStr string
	}{
		{
			name: "success",
		},
		{
			name:           "err",
			setUnlockedErr: errors.New("failed to clear data"),

			expectedErrorStr: "failed to clear data",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			apiClient := &mocks.MetaMorphAPIClientMock{
				SetUnlockedByNameFunc: func(ctx context.Context, in *metamorph_api.SetUnlockedByNameRequest, opts ...grpc.CallOption) (*metamorph_api.SetUnlockedByNameResponse, error) {
					return &metamorph_api.SetUnlockedByNameResponse{RecordsAffected: 5}, tc.setUnlockedErr
				},
			}

			client := metamorph.NewClient(apiClient)

			res, err := client.SetUnlockedByName(context.Background(), "test-1")
			if tc.expectedErrorStr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.expectedErrorStr)
				return
			}

			require.Equal(t, int64(5), res)
		})
	}
}
