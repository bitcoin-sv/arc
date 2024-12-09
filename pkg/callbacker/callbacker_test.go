package callbacker_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
	"github.com/bitcoin-sv/arc/internal/callbacker/mocks"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/bitcoin-sv/arc/pkg/callbacker"
)

func TestSendCallback(t *testing.T) {
	tt := []struct {
		name          string
		expectedCalls int
		err           error
		data          *store.Data
	}{
		{
			name:          "empty callbacks",
			expectedCalls: 0,
			data: &store.Data{
				Status:    metamorph_api.Status_UNKNOWN,
				Hash:      &chainhash.Hash{},
				Callbacks: []store.Callback{},
			},
		},
		{
			name:          "empty url",
			expectedCalls: 0,

			data: &store.Data{
				Status: metamorph_api.Status_UNKNOWN,
				Hash:   &chainhash.Hash{},
				Callbacks: []store.Callback{
					{
						CallbackURL: "",
					},
				},
			},
		},
		{
			name:          "expected call",
			expectedCalls: 1,
			data: &store.Data{
				Status: metamorph_api.Status_UNKNOWN,
				Hash:   &chainhash.Hash{},
				Callbacks: []store.Callback{
					{
						CallbackURL: "http://someurl.comg",
					},
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			apiClient := &mocks.CallbackerAPIClientMock{
				SendCallbackFunc: func(_ context.Context, _ *callbacker_api.SendCallbackRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
					return nil, nil
				},
			}
			grpcCallbacker := callbacker.NewGrpcCallbacker(apiClient, slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
			grpcCallbacker.SendCallback(context.Background(), tc.data)
			require.Equal(t, tc.expectedCalls, len(apiClient.SendCallbackCalls()))
		})
	}
}
