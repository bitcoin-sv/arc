package send_manager_test

import (
	"context"
	"errors"
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

func TestSendManagerStart(t *testing.T) {
	callbackEntries10 := make([]*store.CallbackData, 10)
	for i := range 10 {
		callbackEntries10[i] = &store.CallbackData{}
	}

	tcs := []struct {
		name               string
		callbackData       []*store.CallbackData
		singleSendInterval time.Duration
		batchInterval      time.Duration
		retry              bool
		success            bool
		getAndDeleteErr    error
		commitErr          error
		rollbackErr        error

		expectedCommits        int
		expectedRollbacks      int
		expectedSendCalls      int
		expectedSendBatchCalls int
	}{
		{
			name:               "send 1 single callback",
			callbackData:       []*store.CallbackData{{}},
			singleSendInterval: 10 * time.Millisecond,
			batchInterval:      500 * time.Millisecond,
			retry:              false,
			success:            true,

			expectedCommits:   1,
			expectedSendCalls: 1,
		},
		{
			name:               "send 1 single callback - retry",
			callbackData:       []*store.CallbackData{{}},
			singleSendInterval: 10 * time.Millisecond,
			batchInterval:      500 * time.Millisecond,
			retry:              true,
			success:            false,

			expectedRollbacks: 1,
			expectedSendCalls: 1,
		},
		{
			name:               "send 1 single callback - get error",
			callbackData:       []*store.CallbackData{{}},
			singleSendInterval: 10 * time.Millisecond,
			batchInterval:      500 * time.Millisecond,
			getAndDeleteErr:    errors.New("error"),

			expectedRollbacks: 1,
		},
		{
			name:               "send 1 single callback - commit error",
			callbackData:       []*store.CallbackData{{}},
			singleSendInterval: 10 * time.Millisecond,
			batchInterval:      500 * time.Millisecond,
			retry:              false,
			success:            true,
			commitErr:          errors.New("commit error"),

			expectedCommits:   1,
			expectedRollbacks: 1,
			expectedSendCalls: 1,
		},
		{
			name:               "send 1 single callback - get error - rollback error",
			callbackData:       []*store.CallbackData{{}},
			singleSendInterval: 10 * time.Millisecond,
			batchInterval:      500 * time.Millisecond,
			getAndDeleteErr:    errors.New("error"),
			rollbackErr:        errors.New("rollback error"),

			expectedRollbacks: 1,
		},
		{
			name:               "send 10 batched callbacks",
			callbackData:       callbackEntries10,
			singleSendInterval: 500 * time.Millisecond,
			batchInterval:      10 * time.Millisecond,
			retry:              false,
			success:            true,

			expectedCommits:        1,
			expectedSendBatchCalls: 1,
		},
		{
			name:               "send 10 batched callbacks - retry",
			callbackData:       callbackEntries10,
			singleSendInterval: 500 * time.Millisecond,
			batchInterval:      10 * time.Millisecond,
			retry:              true,
			success:            false,

			expectedRollbacks:      1,
			expectedSendBatchCalls: 1,
		},
		{
			name:               "send 10 batched callbacks - get error",
			callbackData:       callbackEntries10,
			singleSendInterval: 500 * time.Millisecond,
			batchInterval:      10 * time.Millisecond,
			getAndDeleteErr:    errors.New("error"),

			expectedRollbacks: 1,
		},
		{
			name:               "send 10 batched callbacks - commit error",
			callbackData:       callbackEntries10,
			singleSendInterval: 500 * time.Millisecond,
			batchInterval:      10 * time.Millisecond,
			retry:              false,
			success:            true,
			commitErr:          errors.New("commit error"),

			expectedCommits:        1,
			expectedRollbacks:      1,
			expectedSendBatchCalls: 1,
		},
		{
			name:               "send 10 batched callbacks - get error - rollback error",
			callbackData:       callbackEntries10,
			singleSendInterval: 500 * time.Millisecond,
			batchInterval:      10 * time.Millisecond,
			getAndDeleteErr:    errors.New("error"),
			rollbackErr:        errors.New("rollback error"),

			expectedRollbacks: 1,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// given

			stopCh := make(chan struct{}, 5)

			senderMock := &mocks.SenderMock{
				SendFunc: func(_, _ string, _ *callbacker.Callback) (bool, bool) {
					stopCh <- struct{}{}
					return tc.success, tc.retry
				},
				SendBatchFunc: func(_, _ string, _ []*callbacker.Callback) (bool, bool) {
					stopCh <- struct{}{}
					return tc.success, tc.retry
				},
			}

			commitCounter := 0
			commit := func() error {
				commitCounter++
				return tc.commitErr
			}

			rollbackCounter := 0
			rollback := func() error {
				rollbackCounter++
				return tc.rollbackErr
			}
			storeMock := &mocks.SendManagerStoreMock{
				GetAndMarkSentFunc: func(_ context.Context, _ string, _ int, _ time.Duration, _ bool) ([]*store.CallbackData, func() error, func() error, error) {
					if tc.getAndDeleteErr != nil {
						stopCh <- struct{}{}
					}

					return tc.callbackData, commit, rollback, tc.getAndDeleteErr
				},
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

			sut := send_manager.New("https://abcdefg.com", senderMock, storeMock, logger,
				send_manager.WithSingleSendInterval(tc.singleSendInterval),
				send_manager.WithBatchSendInterval(tc.batchInterval),
				send_manager.WithBatchSize(5),
			)

			sut.Start()

			select {
			case <-stopCh:
				sut.GracefulStop()
			case <-time.NewTimer(5 * time.Second).C:
				t.Fatal("Timed out waiting for callbacks to finish")
			}

			assert.Equal(t, tc.expectedSendCalls, len(senderMock.SendCalls()))
			assert.Equal(t, tc.expectedSendBatchCalls, len(senderMock.SendBatchCalls()))
			assert.Equal(t, tc.expectedCommits, commitCounter)
			assert.Equal(t, tc.expectedRollbacks, rollbackCounter)
		})
	}
}

func TestSendManagerStarStore(t *testing.T) {
	tt := []struct {
		name       string
		enqueue    int
		setManyErr error
	}{
		{
			name:    "store 5 callbacks",
			enqueue: 6,
		},
		{
			name:       "store 5 callbacks - set many error",
			enqueue:    6,
			setManyErr: errors.New("error"),
		},
		{
			name:    "store 4 callbacks",
			enqueue: 4,
		},
		{
			name:       "store 4 callbacks - set many error",
			enqueue:    4,
			setManyErr: errors.New("error"),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

			stopCh := make(chan struct{}, 5)
			storeMock := &mocks.SendManagerStoreMock{
				SetManyFunc: func(_ context.Context, data []*store.CallbackData) (int64, error) {
					stopCh <- struct{}{}
					return int64(len(data)), tc.setManyErr
				},
			}

			sut := send_manager.New("https://abcdefg.com", nil, storeMock, logger,
				send_manager.WithStoreCallbackBatchSize(5),
				send_manager.WithStoreCallbacksInterval(200*time.Millisecond),
			)

			sut.StartStoreCallbacks()

			callbackEntry := callbacker.CallbackEntry{Data: &callbacker.Callback{}}
			for range tc.enqueue {
				sut.Enqueue(callbackEntry)
			}

			select {
			case <-stopCh:
				sut.GracefulStop()
			case <-time.NewTimer(5 * time.Second).C:
				t.Fatal("Timed out waiting for callbacks to finish")
			}
		})
	}
}
