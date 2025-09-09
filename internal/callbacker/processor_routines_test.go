package callbacker_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/callbacker"
	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
	"github.com/bitcoin-sv/arc/internal/callbacker/mocks"
	"github.com/bitcoin-sv/arc/internal/callbacker/store"
)

func TestStartCallbackStoreCleanup(t *testing.T) {
	tt := []struct {
		name                     string
		deleteFailedOlderThanErr error

		expectedClearCalls int
	}{
		{
			name: "success",

			expectedClearCalls: 1,
		},
		{
			name:                     "error deleting failed older than",
			deleteFailedOlderThanErr: errors.New("some error"),

			expectedClearCalls: 1,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			cbStore := &mocks.ProcessorStoreMock{
				ClearFunc: func(_ context.Context, _ time.Time) error {
					return tc.deleteFailedOlderThanErr
				},
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
			processor, err := callbacker.NewProcessor(nil, cbStore, nil, logger)
			require.NoError(t, err)
			defer processor.GracefulStop()
			callbacker.CallbackStoreCleanup(processor)

			require.Equal(t, tc.expectedClearCalls, len(cbStore.ClearCalls()))
		})
	}
}

func TestSendCallbacks(t *testing.T) {
	tt := []struct {
		name        string
		sendRetry   bool
		sendSuccess bool

		expectedGetUnsentCalls    int
		expectedSenderSendCalls   int
		expectedSetSentCalls      int
		expectedUnsetPendingCalls int
	}{
		{
			name:        "success",
			sendSuccess: true,
			sendRetry:   false,

			expectedGetUnsentCalls:    1,
			expectedSenderSendCalls:   3,
			expectedSetSentCalls:      3,
			expectedUnsetPendingCalls: 0,
		},
		{
			name:        "no success",
			sendSuccess: false,
			sendRetry:   false,

			expectedGetUnsentCalls:    1,
			expectedSenderSendCalls:   3,
			expectedSetSentCalls:      0,
			expectedUnsetPendingCalls: 3,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			cbStore := &mocks.ProcessorStoreMock{
				GetUnsentFunc: func(_ context.Context, _ int, _ time.Duration, _ bool) ([]*store.CallbackData, error) {
					callbackData := []*store.CallbackData{
						{
							ID:         1,
							URL:        "abc-1.com",
							Token:      "1234",
							TxID:       "tx-1",
							TxStatus:   callbacker_api.Status_MINED.String(),
							AllowBatch: false,
						},
						{
							ID:         2,
							URL:        "abc-2.com",
							Token:      "1234",
							TxID:       "tx-2",
							TxStatus:   callbacker_api.Status_MINED.String(),
							AllowBatch: false,
						},
						{
							ID:         3,
							URL:        "abc-2.com",
							Token:      "1234",
							TxID:       "tx-3",
							TxStatus:   callbacker_api.Status_MINED.String(),
							AllowBatch: false,
						},
					}
					return callbackData, nil
				},
				SetSentFunc: func(_ context.Context, _ []int64) error {
					return nil
				},
				UnsetPendingFunc: func(_ context.Context, _ []int64) error {
					return nil
				},
			}

			sender := &mocks.SenderIMock{
				SendFunc: func(_ string, _ string, _ *callbacker.Callback) (bool, bool) {
					return tc.sendSuccess, tc.sendRetry
				},
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
			processor, err := callbacker.NewProcessor(sender, cbStore, nil, logger)
			require.NoError(t, err)
			defer processor.GracefulStop()
			callbacker.LoadAndSendSingleCallbacks(processor)

			require.Equal(t, tc.expectedGetUnsentCalls, len(cbStore.GetUnsentCalls()))
			require.Equal(t, tc.expectedSetSentCalls, len(cbStore.SetSentCalls()))
			require.Equal(t, tc.expectedUnsetPendingCalls, len(cbStore.UnsetPendingCalls()))
			require.Equal(t, tc.expectedSenderSendCalls, len(sender.SendCalls()))
		})
	}
}

func TestSendBatchCallbacks(t *testing.T) {
	tt := []struct {
		name        string
		sendRetry   bool
		sendSuccess bool

		expectedGetUnsentCalls       int
		expectedSenderSendBatchCalls int
		expectedSetSentCalls         int
		expectedUnsetPendingCalls    int
	}{
		{
			name:        "success",
			sendSuccess: true,
			sendRetry:   false,

			expectedGetUnsentCalls:       1,
			expectedSenderSendBatchCalls: 2,
			expectedSetSentCalls:         2,
			expectedUnsetPendingCalls:    0,
		},
		{
			name:        "no success",
			sendSuccess: false,
			sendRetry:   false,

			expectedGetUnsentCalls:       1,
			expectedSenderSendBatchCalls: 2,
			expectedSetSentCalls:         0,
			expectedUnsetPendingCalls:    2,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			cbStore := &mocks.ProcessorStoreMock{
				GetUnsentFunc: func(_ context.Context, _ int, _ time.Duration, _ bool) ([]*store.CallbackData, error) {
					callbackData := []*store.CallbackData{
						{
							ID:         1,
							URL:        "abc-1.com",
							Token:      "1234",
							TxID:       "tx-1",
							TxStatus:   callbacker_api.Status_MINED.String(),
							AllowBatch: false,
						},
						{
							ID:         2,
							URL:        "abc-2.com",
							Token:      "1234",
							TxID:       "tx-2",
							TxStatus:   callbacker_api.Status_MINED.String(),
							AllowBatch: false,
						},
						{
							ID:         3,
							URL:        "abc-2.com",
							Token:      "1234",
							TxID:       "tx-3",
							TxStatus:   callbacker_api.Status_MINED.String(),
							AllowBatch: false,
						},
					}
					return callbackData, nil
				},
				SetSentFunc: func(_ context.Context, _ []int64) error {
					return nil
				},
				UnsetPendingFunc: func(_ context.Context, _ []int64) error {
					return nil
				},
			}

			sender := &mocks.SenderIMock{
				SendBatchFunc: func(_ string, _ string, _ []*callbacker.Callback) (bool, bool) {
					return tc.sendSuccess, tc.sendRetry
				},
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
			processor, err := callbacker.NewProcessor(sender, cbStore, nil, logger)
			require.NoError(t, err)
			defer processor.GracefulStop()
			callbacker.LoadAndSendBatchCallbacks(processor)

			assert.Equal(t, tc.expectedGetUnsentCalls, len(cbStore.GetUnsentCalls()))
			assert.Equal(t, tc.expectedSetSentCalls, len(cbStore.SetSentCalls()))
			assert.Equal(t, tc.expectedUnsetPendingCalls, len(cbStore.UnsetPendingCalls()))
			assert.Equal(t, tc.expectedSenderSendBatchCalls, len(sender.SendBatchCalls()))
		})
	}
}
