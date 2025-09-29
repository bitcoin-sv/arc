package callbacker_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/bitcoin-sv/arc/internal/callbacker"
	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
	"github.com/bitcoin-sv/arc/internal/callbacker/mocks"
	"github.com/bitcoin-sv/arc/internal/callbacker/store"
	"github.com/bitcoin-sv/arc/internal/grpc_utils"
	mqMocks "github.com/bitcoin-sv/arc/internal/mq/mocks"
)

func TestNewServer(t *testing.T) {
	t.Run("successfully creates a new server", func(t *testing.T) {
		// Given

		// When
		server, err := callbacker.NewServer(slog.Default(), nil, nil, grpc_utils.ServerConfig{})

		// Then
		require.NoError(t, err)
		require.NotNil(t, server)
		assert.IsType(t, &callbacker.Server{}, server)

		defer server.GracefulStop()
	})
}

func TestHealth(t *testing.T) {
	t.Run("returns the current health with a valid timestamp", func(t *testing.T) {
		mqClient := &mqMocks.MessageQueueClientMock{
			StatusFunc: func() nats.Status {
				return nats.CONNECTED
			},
		}

		// Given
		sut, err := callbacker.NewServer(slog.Default(), nil, mqClient, grpc_utils.ServerConfig{})
		require.NoError(t, err)
		defer sut.GracefulStop()

		// When
		stats, err := sut.Health(context.Background(), &emptypb.Empty{})

		// Then
		assert.NoError(t, err)
		require.NotNil(t, stats)
		assert.NotNil(t, stats.Timestamp)
		assert.Equal(t, nats.CONNECTED.String(), stats.Nats)

		now := time.Now().Unix()
		assert.InDelta(t, now, stats.Timestamp.Seconds, 1, "Timestamp should be close to the current time")
	})
}

func TestSendCallback(t *testing.T) {
	t.Run("dispatches callback for each routing", func(t *testing.T) {
		// Given

		callbackerStore := &mocks.ProcessorStoreMock{
			InsertFunc: func(_ context.Context, _ []*store.CallbackData) (int64, error) {
				return 3, nil
			},
		}

		server, err := callbacker.NewServer(slog.Default(), callbackerStore, nil, grpc_utils.ServerConfig{})
		require.NoError(t, err)

		request := &callbacker_api.SendRequest{
			Txid:            "1234",
			Status:          callbacker_api.Status_SEEN_ON_NETWORK,
			CallbackRouting: &callbacker_api.CallbackRouting{Url: "https://example.com/callback1", Token: "token1", AllowBatch: false},
			BlockHash:       "abcd1234",
			BlockHeight:     100,
			MerklePath:      "path/to/merkle",
			ExtraInfo:       "extra info",
			CompetingTxs:    []string{"tx1", "tx2"},
		}

		// When
		resp, err := server.SendCallback(context.Background(), request)

		// Then
		assert.NoError(t, err)
		assert.IsType(t, &emptypb.Empty{}, resp)
		time.Sleep(100 * time.Millisecond)
	})
}
