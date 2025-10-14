package callbacker_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/bsv-blockchain/go-bt/v2/chainhash"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/bitcoin-sv/arc/internal/callbacker"
	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
	"github.com/bitcoin-sv/arc/internal/callbacker/mocks"
	"github.com/bitcoin-sv/arc/internal/global"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
)

func TestSendGRPCCallback(t *testing.T) {
	tt := []struct {
		name          string
		expectedCalls int
		err           error
		data          *global.TransactionData
	}{
		{
			name:          "empty callbacks",
			expectedCalls: 0,
			data: &global.TransactionData{
				Status:    metamorph_api.Status_UNKNOWN,
				Hash:      &chainhash.Hash{},
				Callbacks: []global.Callback{},
			},
		},
		{
			name:          "empty url",
			expectedCalls: 0,

			data: &global.TransactionData{
				Status: metamorph_api.Status_UNKNOWN,
				Hash:   &chainhash.Hash{},
				Callbacks: []global.Callback{
					{
						CallbackURL: "",
					},
				},
			},
		},
		{
			name:          "expected call",
			expectedCalls: 1,
			data: &global.TransactionData{
				Status: metamorph_api.Status_UNKNOWN,
				Hash:   &chainhash.Hash{},
				Callbacks: []global.Callback{
					{
						CallbackURL: "http://someurl.comg",
					},
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			apiClient := &mocks.CallbackerAPIClientMock{
				SendCallbackFunc: func(_ context.Context, _ *callbacker_api.SendRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
					return nil, nil
				},
			}
			sut := callbacker.NewGrpcCallbacker(apiClient, slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

			// when
			sut.SendCallback(context.Background(), tc.data)

			// then
			require.Equal(t, tc.expectedCalls, len(apiClient.SendCallbackCalls()))
		})
	}
}
