package callbacker_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/bitcoin-sv/arc/internal/callbacker"
	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
	"github.com/bitcoin-sv/arc/internal/callbacker/mocks"
	"github.com/bitcoin-sv/arc/internal/callbacker/store"
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
		sendOK := true
		senderMq := &mocks.SenderIMock{
			SendFunc:      func(_, _ string, _ *callbacker.Callback) bool { return sendOK },
			SendBatchFunc: func(_, _ string, _ []*callbacker.Callback) bool { return sendOK },
		}

		storeMq := &mocks.CallbackerStoreMock{
			SetFunc: func(_ context.Context, _ *store.CallbackData) error {
				return nil
			},
			SetManyFunc: func(_ context.Context, _ []*store.CallbackData) error {
				return nil
			},
		}

		mockDispatcher := callbacker.NewCallbackDispatcher(
			senderMq,
			storeMq,
			slog.Default(),
			&callbacker.SendConfig{Expiration: time.Duration(24 * time.Hour)},
		)

		server, err := callbacker.NewServer("", 0, slog.Default(), mockDispatcher, nil)
		require.NoError(t, err)

		request := &callbacker_api.SendCallbackRequest{
			Txid:   "1234",
			Status: callbacker_api.Status_SEEN_ON_NETWORK,
			CallbackRoutings: []*callbacker_api.CallbackRouting{
				{Url: "http://example.com/callback1", Token: "token1", AllowBatch: false},
				{Url: "http://example.com/callback2", Token: "token2", AllowBatch: false},
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
		time.Sleep(100 * time.Millisecond)
		require.Equal(t, 2, len(senderMq.SendCalls()), "Expected two dispatch calls")

		calls0 := senderMq.SendCalls()[0]
		calls1 := senderMq.SendCalls()[1]

		switch calls0.URL {
		case "http://example.com/callback1":
			require.Equal(t, calls0.Token, "token1")
			require.Equal(t, calls1.URL, "http://example.com/callback2")
			require.Equal(t, calls1.Token, "token2")
		case "http://example.com/callback2":
			require.Equal(t, calls0.Token, "token2")
			require.Equal(t, calls1.URL, "http://example.com/callback1")
			require.Equal(t, calls1.Token, "token1")
		default:
			t.Fatalf("unexpected callback URL: %s", calls0.URL)
		}

		mockDispatcher.GracefulStop()
	})
}
