package ordered_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/callbacker"
	"github.com/bitcoin-sv/arc/internal/callbacker/send_manager/ordered"
	"github.com/bitcoin-sv/arc/internal/callbacker/send_manager/ordered/mocks"
	"github.com/bitcoin-sv/arc/internal/callbacker/store"
)

func TestSendManagerStart(t *testing.T) {
	tcs := []struct {
		name               string
		callbacksEnqueued  int
		singleSendInterval time.Duration
		backfillInterval   time.Duration
		callbackTimestamp  time.Time

		expectedCallbacksEnqueued int
		expectedSetManyCalls      int
		expectedSetCalls          int
		expectedSendCalls         int
	}{
		{
			name:               "enqueue 10 callbacks - 10ms interval",
			callbacksEnqueued:  10,
			singleSendInterval: 10 * time.Millisecond,
			backfillInterval:   500 * time.Millisecond,
			callbackTimestamp:  time.Date(2025, 1, 10, 11, 30, 0, 0, time.UTC),

			expectedCallbacksEnqueued: 10,
			expectedSetManyCalls:      0,
			expectedSetCalls:          0,
			expectedSendCalls:         10,
		},
		{
			name:               "enqueue 10 callbacks - 100ms interval - store remaining at graceful stop",
			callbacksEnqueued:  10,
			singleSendInterval: 100 * time.Millisecond,
			backfillInterval:   500 * time.Millisecond,
			callbackTimestamp:  time.Date(2025, 1, 10, 11, 30, 0, 0, time.UTC),

			expectedCallbacksEnqueued: 10,
			expectedSetManyCalls:      1,
			expectedSetCalls:          0,
			expectedSendCalls:         1,
		},
		{
			name:               "enqueue 10 callbacks - expired",
			callbacksEnqueued:  10,
			singleSendInterval: 10 * time.Millisecond,
			backfillInterval:   500 * time.Millisecond,
			callbackTimestamp:  time.Date(2025, 1, 9, 12, 0, 0, 0, time.UTC),

			expectedCallbacksEnqueued: 10,
			expectedSetManyCalls:      0,
			expectedSetCalls:          0,
			expectedSendCalls:         0,
		},
		{
			name:               "enqueue 15 callbacks - buffer size reached",
			callbacksEnqueued:  15,
			singleSendInterval: 10 * time.Millisecond,
			backfillInterval:   500 * time.Millisecond,
			callbackTimestamp:  time.Date(2025, 1, 10, 11, 30, 0, 0, time.UTC),

			expectedCallbacksEnqueued: 10,
			expectedSetManyCalls:      0,
			expectedSetCalls:          5,
			expectedSendCalls:         10,
		},
		{
			name:               "enqueue 10 callbacks - 10ms interval - back fill queue",
			callbacksEnqueued:  10,
			singleSendInterval: 10 * time.Millisecond,
			backfillInterval:   20 * time.Millisecond,
			callbackTimestamp:  time.Date(2025, 1, 10, 11, 30, 0, 0, time.UTC),

			expectedCallbacksEnqueued: 10,
			expectedSetManyCalls:      1,
			expectedSetCalls:          0,
			expectedSendCalls:         15,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// given
			senderMock := &mocks.SenderMock{
				SendFunc:      func(_, _ string, _ *callbacker.Callback) (bool, bool) { return true, false },
				SendBatchFunc: func(_, _ string, _ []*callbacker.Callback) (bool, bool) { return true, false },
			}

			storeMock := &mocks.SendManagerStoreMock{
				SetManyFunc: func(_ context.Context, data []*store.CallbackData) error {
					return nil
				},
				SetFunc: func(_ context.Context, data *store.CallbackData) error {
					return nil
				},
				GetAndDeleteFunc: func(ctx context.Context, url string, limit int) ([]*store.CallbackData, error) {
					var callbacks []*store.CallbackData
					for range limit {
						callbacks = append(callbacks, &store.CallbackData{Timestamp: tc.callbackTimestamp})
					}
					return callbacks, nil
				},
			}

			sut := ordered.New("https://abcdefg.com", senderMock, storeMock, slog.Default(),
				ordered.WithBufferSize(10),
				ordered.WithNow(func() time.Time {
					return time.Date(2025, 1, 10, 12, 0, 0, 0, time.UTC)
				}),
				ordered.WithSingleSendInterval(tc.singleSendInterval),
				ordered.WithExpiration(time.Hour),
				ordered.WithBackfillQueueInterval(tc.backfillInterval),
			)

			// add callbacks before starting the manager to queue them
			for range tc.callbacksEnqueued {
				sut.Enqueue(callbacker.CallbackEntry{Data: &callbacker.Callback{
					Timestamp: tc.callbackTimestamp,
				}})
			}
			require.Equal(t, tc.expectedCallbacksEnqueued, sut.CallbacksQueued())

			sut.Start()

			time.Sleep(150 * time.Millisecond)
			sut.GracefulStop()

			assert.Equal(t, 0, sut.CallbacksQueued())
			assert.Equal(t, tc.expectedSetManyCalls, len(storeMock.SetManyCalls()))
			assert.Equal(t, tc.expectedSetCalls, len(storeMock.SetCalls()))
			assert.Equal(t, tc.expectedSendCalls, len(senderMock.SendCalls()))
		})
	}
}
