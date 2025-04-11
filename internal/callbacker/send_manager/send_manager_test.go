package send_manager_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/bitcoin-sv/arc/internal/callbacker"
	"github.com/bitcoin-sv/arc/internal/callbacker/send_manager"
	"github.com/bitcoin-sv/arc/internal/callbacker/send_manager/mocks"
	"github.com/bitcoin-sv/arc/internal/callbacker/store"
)

var (
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

	callbackEntriesUnsorted := []callbacker.CallbackEntry{
		{Data: &callbacker.Callback{Timestamp: time.Date(2025, 1, 12, 24, 59, 0, 0, time.UTC)}},
		{Data: &callbacker.Callback{Timestamp: time.Date(2025, 1, 11, 24, 59, 0, 0, time.UTC)}},
		{Data: &callbacker.Callback{Timestamp: time.Date(2025, 1, 31, 24, 59, 0, 0, time.UTC)}},
		{Data: &callbacker.Callback{Timestamp: time.Date(2025, 1, 17, 24, 59, 0, 0, time.UTC)}},
		{Data: &callbacker.Callback{Timestamp: time.Date(2025, 1, 13, 24, 59, 0, 0, time.UTC)}},
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
			queueProcessInterval:    18 * time.Millisecond,
			backfillInterval:        500 * time.Millisecond,
			sortByTimestampInterval: 10 * time.Millisecond,
			batchInterval:           500 * time.Millisecond,

			expectedCallbacksEnqueued: 5,
			expectedSetMany:           0,
			expectedSetCalls:          0,
			expectedSendCalls:         5,
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
			var lastData *callbacker.Callback
			senderMock := &mocks.SenderMock{
				SendFunc: func(_, _ string, data *callbacker.Callback) (bool, bool) {
					if lastData != nil {
						assert.LessOrEqual(t, lastData.Timestamp, data.Timestamp)
					}
					lastData = data
					return true, false
				},
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
						assert.LessOrEqual(t, data[i].Timestamp, data[i+1].Timestamp)
					}

					return nil
				},
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

			sut := send_manager.New("https://abcdefg.com", senderMock, storeMock, logger,
				send_manager.WithQueueProcessInterval(tc.queueProcessInterval),
				send_manager.WithExpiration(time.Hour),
				send_manager.WithBatchSendInterval(tc.batchInterval),
				send_manager.WithBatchSize(5),
			)

			// add callbacks before starting the manager to queue them
			for _, cb := range tc.callbacksEnqueued {
				sut.Enqueue(cb)
			}

			sut.Start()

			time.Sleep(150 * time.Millisecond)
			sut.GracefulStop()

			assert.Equal(t, tc.expectedSetMany > 0, len(storeMock.SetManyCalls()) == 1)
			assert.Equal(t, tc.expectedSendCalls, len(senderMock.SendCalls()))
			assert.Equal(t, len(tc.expectedSendBatchCalls), len(senderMock.SendBatchCalls()))
		})
	}
}
