package callbacker_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/callbacker"
	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestNewServer(t *testing.T) {
	t.Run("successfully creates a new server", func(t *testing.T) {
		// Given
		logger := slog.Default()

		// When
		server, err := callbacker.NewServer("", 0, logger, nil, nil)

		// Then
		require.NoError(t, err)
		require.NotNil(t, server)
		assert.IsType(t, &callbacker.Server{}, server)

		defer server.GracefulStop()
	})
}

func TestHealth(t *testing.T) {
	t.Run("returns the current health with a valid timestamp", func(t *testing.T) {
		// Given
		sut, err := callbacker.NewServer("", 0, slog.Default(), nil, nil)
		require.NoError(t, err)
		defer sut.GracefulStop()

		// When
		stats, err := sut.Health(context.Background(), &emptypb.Empty{})

		// Then
		assert.NoError(t, err)
		require.NotNil(t, stats)
		assert.NotNil(t, stats.Timestamp)

		now := time.Now().Unix()
		assert.InDelta(t, now, stats.Timestamp.Seconds, 1, "Timestamp should be close to the current time")
	})
}

func TestSendCallback(t *testing.T) {
	t.Run("dispatches callback for each routing", func(t *testing.T) {
		// Given
		mockDispatcher := callbacker.NewCallbackDispatcher(
			&callbacker.SenderIMock{
				SendFunc: func(_, _ string, _ *callbacker.Callback) bool { return true },
			},
			nil, // Mock store is not needed for this test
			slog.Default(),
			0, 0, 0, 0,
		)

		server, err := callbacker.NewServer("", 0, slog.Default(), mockDispatcher, nil)
		require.NoError(t, err)

		request := &callbacker_api.SendCallbackRequest{
			Txid:   "1234",
			Status: callbacker_api.Status_SEEN_ON_NETWORK,
			CallbackRoutings: []*callbacker_api.CallbackRouting{
				{Url: "http://example.com/callback1", Token: "token1", AllowBatch: false},
				{Url: "http://example.com/callback2", Token: "token2", AllowBatch: true},
			},
			BlockHash:    "abcd1234",
			BlockHeight:  100,
			MerklePath:   "path/to/merkle",
			ExtraInfo:    "extra info",
			CompetingTxs: []string{"tx1", "tx2"},
		}

		// When
		resp, err := server.SendCallback(context.Background(), request)

		// Then
		assert.NoError(t, err)
		assert.IsType(t, &emptypb.Empty{}, resp)

		for _, routing := range request.CallbackRoutings {
			assert.NotEmpty(t, routing.Url)
			assert.NotEmpty(t, routing.Token)
		}
	})
}
