package ordered_test

import (
	"context"
	"log/slog"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/callbacker"
	"github.com/bitcoin-sv/arc/internal/callbacker/send_manager/ordered"
	"github.com/bitcoin-sv/arc/internal/callbacker/send_manager/ordered/mocks"
	"github.com/bitcoin-sv/arc/internal/callbacker/store"
)

var (
	now       = time.Date(2025, 1, 10, 12, 0, 0, 0, time.UTC)
	ts        = time.Date(2025, 1, 10, 11, 30, 0, 0, time.UTC)
	tsExpired = time.Date(2025, 1, 9, 12, 0, 0, 0, time.UTC)
)

func TestSendManagerStart(t *testing.T) {
	callbackEntries10 := make([]callbacker.CallbackEntry, 10)
	for i := range 10 {
		callbackEntries10[i] = callbacker.CallbackEntry{Data: &callbacker.Callback{Timestamp: ts}}
	}

	callbackEntries15 := make([]callbacker.CallbackEntry, 15)
	for i := range 15 {
		callbackEntries15[i] = callbacker.CallbackEntry{Data: &callbacker.Callback{Timestamp: ts}}
	}

	callbackEntries10Expired := make([]callbacker.CallbackEntry, 10)
	for i := range 10 {
		callbackEntries10Expired[i] = callbacker.CallbackEntry{Data: &callbacker.Callback{Timestamp: tsExpired}}
	}

	callbackEntriesUnsorted := make([]callbacker.CallbackEntry, 10)
	for i := range 10 {
		callbackEntriesUnsorted[i] = callbacker.CallbackEntry{Data: &callbacker.Callback{Timestamp: time.Date(2025, 1, rand.Intn(31), rand.Intn(24), rand.Intn(59), 0, 0, time.UTC)}}
	}

	tcs := []struct {
		name                    string
		callbacksEnqueued       []callbacker.CallbackEntry
		singleSendInterval      time.Duration
		backfillInterval        time.Duration
		sortByTimestampInterval time.Duration

		expectedCallbacksEnqueued int
		expectedSetManyCalls      int
		expectedSetCalls          int
		expectedSendCalls         int
	}{
		{
			name:                    "enqueue 10 callbacks - 10ms interval",
			callbacksEnqueued:       callbackEntries10,
			singleSendInterval:      10 * time.Millisecond,
			backfillInterval:        500 * time.Millisecond,
			sortByTimestampInterval: 500 * time.Millisecond,

			expectedCallbacksEnqueued: 10,
			expectedSetManyCalls:      0,
			expectedSetCalls:          0,
			expectedSendCalls:         10,
		},
		{
			name:                    "enqueue 10 callbacks - 100ms interval - store remaining at graceful stop",
			callbacksEnqueued:       callbackEntries10,
			singleSendInterval:      100 * time.Millisecond,
			backfillInterval:        500 * time.Millisecond,
			sortByTimestampInterval: 500 * time.Millisecond,

			expectedCallbacksEnqueued: 10,
			expectedSetManyCalls:      1,
			expectedSetCalls:          0,
			expectedSendCalls:         1,
		},
		{
			name:                    "enqueue 10 callbacks - expired",
			callbacksEnqueued:       callbackEntries10Expired,
			singleSendInterval:      10 * time.Millisecond,
			backfillInterval:        500 * time.Millisecond,
			sortByTimestampInterval: 500 * time.Millisecond,

			expectedCallbacksEnqueued: 10,
			expectedSetManyCalls:      0,
			expectedSetCalls:          0,
			expectedSendCalls:         0,
		},
		{
			name:                    "enqueue 15 callbacks - buffer size reached",
			callbacksEnqueued:       callbackEntries15,
			singleSendInterval:      10 * time.Millisecond,
			backfillInterval:        500 * time.Millisecond,
			sortByTimestampInterval: 500 * time.Millisecond,

			expectedCallbacksEnqueued: 10,
			expectedSetManyCalls:      0,
			expectedSetCalls:          5,
			expectedSendCalls:         10,
		},
		{
			name:                    "enqueue 10 callbacks - 10ms interval - back fill queue",
			callbacksEnqueued:       callbackEntries10,
			singleSendInterval:      10 * time.Millisecond,
			backfillInterval:        20 * time.Millisecond,
			sortByTimestampInterval: 500 * time.Millisecond,

			expectedCallbacksEnqueued: 10,
			expectedSetManyCalls:      1,
			expectedSetCalls:          0,
			expectedSendCalls:         15,
		},
		{
			name:                    "enqueue 10 callbacks - 10ms interval - sort by timestamp",
			callbacksEnqueued:       callbackEntriesUnsorted,
			singleSendInterval:      200 * time.Millisecond,
			backfillInterval:        500 * time.Millisecond,
			sortByTimestampInterval: 20 * time.Millisecond,

			expectedCallbacksEnqueued: 10,
			expectedSetManyCalls:      1,
			expectedSetCalls:          0,
			expectedSendCalls:         0,
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
					for i := 0; i < len(data)-1; i++ {
						assert.GreaterOrEqual(t, data[i].Timestamp, data[i+1].Timestamp)
					}

					return nil
				},
				SetFunc: func(_ context.Context, data *store.CallbackData) error {
					return nil
				},
				GetAndDeleteFunc: func(ctx context.Context, url string, limit int) ([]*store.CallbackData, error) {
					var callbacks []*store.CallbackData
					for range limit {
						callbacks = append(callbacks, &store.CallbackData{Timestamp: time.Date(2025, 1, 10, 11, 30, 0, 0, time.UTC)})
					}
					return callbacks, nil
				},
			}

			sut := ordered.New("https://abcdefg.com", senderMock, storeMock, slog.Default(),
				ordered.WithBufferSize(10),
				ordered.WithNow(func() time.Time {
					return now
				}),
				ordered.WithSingleSendInterval(tc.singleSendInterval),
				ordered.WithExpiration(time.Hour),
				ordered.WithBackfillQueueInterval(tc.backfillInterval),
				ordered.WithSortByTimestampInterval(tc.sortByTimestampInterval),
			)

			// add callbacks before starting the manager to queue them
			for _, cb := range tc.callbacksEnqueued {
				sut.Enqueue(cb)
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
