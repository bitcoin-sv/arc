package ordered_test

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
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

	callbackEntriesBatch3Expired10 := make([]callbacker.CallbackEntry, 10)
	for i := range 10 {
		if i >= 3 && i < 6 {
			callbackEntriesBatch3Expired10[i] = callbacker.CallbackEntry{Token: fmt.Sprintf("token: %d", i), Data: &callbacker.Callback{Timestamp: tsExpired}, AllowBatch: true}
			continue
		}
		callbackEntriesBatch3Expired10[i] = callbacker.CallbackEntry{Token: fmt.Sprintf("token: %d", i), Data: &callbacker.Callback{Timestamp: ts}, AllowBatch: true}
	}

	callbackEntriesBatch10 := make([]callbacker.CallbackEntry, 10)
	for i := range 10 {
		callbackEntriesBatch10[i] = callbacker.CallbackEntry{Token: fmt.Sprintf("token: %d", i), Data: &callbacker.Callback{Timestamp: ts}, AllowBatch: true}
	}

	callbackEntriesBatched10Mixed := make([]callbacker.CallbackEntry, 10)
	for i := range 10 {
		if i >= 3 && i < 7 {
			callbackEntriesBatched10Mixed[i] = callbacker.CallbackEntry{Token: fmt.Sprintf("token: %d", i), Data: &callbacker.Callback{Timestamp: ts}, AllowBatch: true}
			continue
		}
		callbackEntriesBatched10Mixed[i] = callbacker.CallbackEntry{Token: fmt.Sprintf("token: %d", i), Data: &callbacker.Callback{Timestamp: ts}, AllowBatch: false}
	}

	callbackEntriesBatched10Mixed2 := make([]callbacker.CallbackEntry, 10)
	for i := range 10 {
		if i >= 2 && i < 8 {
			callbackEntriesBatched10Mixed2[i] = callbacker.CallbackEntry{Token: fmt.Sprintf("token: %d", i), Data: &callbacker.Callback{Timestamp: ts}, AllowBatch: true}
			continue
		}
		callbackEntriesBatched10Mixed2[i] = callbacker.CallbackEntry{Token: fmt.Sprintf("token: %d", i), Data: &callbacker.Callback{Timestamp: ts}, AllowBatch: false}
	}

	tcs := []struct {
		name                    string
		callbacksEnqueued       []callbacker.CallbackEntry
		queueProcessInterval    time.Duration
		backfillInterval        time.Duration
		sortByTimestampInterval time.Duration
		batchInterval           time.Duration

		expectedCallbacksEnqueued int
		expectedSetMany           int
		expectedSetCalls          int
		expectedSendCalls         int
		expectedSendBatchCalls    []int
	}{
		{
			name:                    "enqueue 10 callbacks - 10ms interval",
			callbacksEnqueued:       callbackEntries10,
			queueProcessInterval:    10 * time.Millisecond,
			backfillInterval:        500 * time.Millisecond,
			sortByTimestampInterval: 500 * time.Millisecond,
			batchInterval:           500 * time.Millisecond,

			expectedCallbacksEnqueued: 10,
			expectedSetCalls:          0,
			expectedSendCalls:         10,
		},
		{
			name:                    "enqueue 10 callbacks - 100ms interval - store remaining at graceful stop",
			callbacksEnqueued:       callbackEntries10,
			queueProcessInterval:    100 * time.Millisecond,
			backfillInterval:        500 * time.Millisecond,
			sortByTimestampInterval: 500 * time.Millisecond,
			batchInterval:           500 * time.Millisecond,

			expectedCallbacksEnqueued: 10,
			expectedSetMany:           9,
			expectedSetCalls:          0,
			expectedSendCalls:         1,
		},
		{
			name:                    "enqueue 10 callbacks - expired",
			callbacksEnqueued:       callbackEntries10Expired,
			queueProcessInterval:    10 * time.Millisecond,
			backfillInterval:        500 * time.Millisecond,
			sortByTimestampInterval: 500 * time.Millisecond,
			batchInterval:           500 * time.Millisecond,

			expectedCallbacksEnqueued: 10,
			expectedSetCalls:          0,
			expectedSendCalls:         0,
		},
		{
			name:                    "enqueue 15 callbacks - buffer size reached",
			callbacksEnqueued:       callbackEntries15,
			queueProcessInterval:    10 * time.Millisecond,
			backfillInterval:        500 * time.Millisecond,
			sortByTimestampInterval: 500 * time.Millisecond,
			batchInterval:           500 * time.Millisecond,

			expectedCallbacksEnqueued: 10,
			expectedSetCalls:          5,
			expectedSendCalls:         10,
		},
		{
			name:                    "enqueue 10 callbacks - back fill queue",
			callbacksEnqueued:       callbackEntries10,
			queueProcessInterval:    10 * time.Millisecond,
			backfillInterval:        20 * time.Millisecond,
			sortByTimestampInterval: 500 * time.Millisecond,
			batchInterval:           500 * time.Millisecond,

			expectedCallbacksEnqueued: 10,
			expectedSetMany:           9,
			expectedSetCalls:          0,
			expectedSendCalls:         15,
		},
		{
			name:                    "enqueue 10 callbacks - sort by timestamp",
			callbacksEnqueued:       callbackEntriesUnsorted,
			queueProcessInterval:    200 * time.Millisecond,
			backfillInterval:        500 * time.Millisecond,
			sortByTimestampInterval: 20 * time.Millisecond,
			batchInterval:           500 * time.Millisecond,

			expectedCallbacksEnqueued: 10,
			expectedSetMany:           10,
			expectedSetCalls:          0,
			expectedSendCalls:         0,
		},
		{
			name:                    "enqueue 10 batched callbacks - 3 expired",
			callbacksEnqueued:       callbackEntriesBatch3Expired10,
			queueProcessInterval:    10 * time.Millisecond,
			backfillInterval:        500 * time.Millisecond,
			sortByTimestampInterval: 500 * time.Millisecond,
			batchInterval:           500 * time.Millisecond,

			expectedCallbacksEnqueued: 10,
			expectedSetMany:           2,
			expectedSetCalls:          0,
			expectedSendCalls:         0,
			expectedSendBatchCalls:    []int{5},
		},
		{
			name:                    "enqueue 10 batched callbacks - 60ms batch interval",
			callbacksEnqueued:       callbackEntriesBatch10,
			queueProcessInterval:    10 * time.Millisecond,
			backfillInterval:        500 * time.Millisecond,
			sortByTimestampInterval: 500 * time.Millisecond,
			batchInterval:           60 * time.Millisecond,

			expectedCallbacksEnqueued: 10,
			expectedSetCalls:          0,
			expectedSendCalls:         0,
			expectedSendBatchCalls:    []int{5, 5},
		},
		{
			name:                    "enqueue 10 batched callbacks - 4 batched, 6 single",
			callbacksEnqueued:       callbackEntriesBatched10Mixed,
			queueProcessInterval:    10 * time.Millisecond,
			backfillInterval:        500 * time.Millisecond,
			sortByTimestampInterval: 500 * time.Millisecond,
			batchInterval:           500 * time.Millisecond,

			expectedCallbacksEnqueued: 10,
			expectedSetCalls:          0,
			expectedSendCalls:         6,
			expectedSendBatchCalls:    []int{4},
		},
		{
			name:                    "enqueue 10 batched callbacks - 6 batched, 4 single",
			callbacksEnqueued:       callbackEntriesBatched10Mixed2,
			queueProcessInterval:    10 * time.Millisecond,
			backfillInterval:        500 * time.Millisecond,
			sortByTimestampInterval: 500 * time.Millisecond,
			batchInterval:           500 * time.Millisecond,

			expectedCallbacksEnqueued: 10,
			expectedSetCalls:          0,
			expectedSendCalls:         4,
			expectedSendBatchCalls:    []int{5, 1},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// given

			counter := 0
			senderMock := &mocks.SenderMock{
				SendFunc: func(_, _ string, _ *callbacker.Callback) (bool, bool) { return true, false },
				SendBatchFunc: func(_, _ string, batch []*callbacker.Callback) (bool, bool) {
					if counter >= len(tc.expectedSendBatchCalls) {
						t.Fail()
					} else {
						assert.Equal(t, tc.expectedSendBatchCalls[counter], len(batch))
						counter++
					}
					return true, false
				},
			}

			storeMock := &mocks.SendManagerStoreMock{
				SetManyFunc: func(_ context.Context, data []*store.CallbackData) error {
					assert.Equal(t, tc.expectedSetMany, len(data))

					for i := 0; i < len(data)-1; i++ {
						assert.GreaterOrEqual(t, data[i].Timestamp, data[i+1].Timestamp)
					}

					return nil
				},
				SetFunc: func(_ context.Context, _ *store.CallbackData) error {
					return nil
				},
				GetAndDeleteFunc: func(_ context.Context, _ string, limit int) ([]*store.CallbackData, error) {
					var callbacks []*store.CallbackData
					for range limit {
						callbacks = append(callbacks, &store.CallbackData{Timestamp: time.Date(2025, 1, 10, 11, 30, 0, 0, time.UTC)})
					}
					return callbacks, nil
				},
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

			sut := ordered.New("https://abcdefg.com", senderMock, storeMock, logger,
				ordered.WithBufferSize(10),
				ordered.WithNow(func() time.Time {
					return now
				}),
				ordered.WithQueueProcessInterval(tc.queueProcessInterval),
				ordered.WithExpiration(time.Hour),
				ordered.WithBackfillQueueInterval(tc.backfillInterval),
				ordered.WithSortByTimestampInterval(tc.sortByTimestampInterval),
				ordered.WithBatchSendInterval(tc.batchInterval),
				ordered.WithBatchSize(5),
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
			assert.Equal(t, tc.expectedSetMany > 0, len(storeMock.SetManyCalls()) == 1)
			assert.Equal(t, tc.expectedSetCalls, len(storeMock.SetCalls()))
			assert.Equal(t, tc.expectedSendCalls, len(senderMock.SendCalls()))
			assert.Equal(t, len(tc.expectedSendBatchCalls), len(senderMock.SendBatchCalls()))
		})
	}
}
