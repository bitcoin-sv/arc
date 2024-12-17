package callbacker_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/callbacker"
	"github.com/bitcoin-sv/arc/internal/callbacker/mocks"
	"github.com/bitcoin-sv/arc/internal/callbacker/store"
)

func TestSendManager(t *testing.T) {
	tcs := []struct {
		name                  string
		singleSendInterval    time.Duration
		numOfSingleCallbacks  int
		numOfBatchedCallbacks int
		stopManager           bool
		setEntriesBufferSize  int
		setErr                error
	}{
		{
			name:                  "send only single callbacks when run",
			singleSendInterval:    0,
			numOfSingleCallbacks:  100,
			numOfBatchedCallbacks: 0,
		},
		{
			name:                  "save callbacks on stopping (only single callbacks)",
			singleSendInterval:    time.Millisecond, // set interval to give time to call stop function
			numOfSingleCallbacks:  10,
			numOfBatchedCallbacks: 0,
			stopManager:           true,
		},
		{
			name:                  "send only batched callbacks when run",
			numOfSingleCallbacks:  0,
			numOfBatchedCallbacks: 493,
		},
		{
			name:                  "save callbacks on stopping (only batched callbacks)",
			numOfSingleCallbacks:  0,
			numOfBatchedCallbacks: 451,
			stopManager:           true,
		},
		{
			name:                  "send mixed callbacks when run",
			singleSendInterval:    0,
			numOfSingleCallbacks:  100,
			numOfBatchedCallbacks: 501,
		},
		{
			name:                  "entry buffer size exceeds limit",
			singleSendInterval:    0,
			numOfSingleCallbacks:  10,
			numOfBatchedCallbacks: 10,
			setEntriesBufferSize:  5,
			stopManager:           true,
		},
		{
			name:                  "entry buffer size exceeds limit - set error",
			singleSendInterval:    0,
			numOfSingleCallbacks:  10,
			numOfBatchedCallbacks: 10,
			setEntriesBufferSize:  5,
			setErr:                errors.New("failed to set entry"),
			stopManager:           true,
		},
		{
			name:                  "save callbacks on stopping (mixed callbacks)",
			singleSendInterval:    time.Millisecond, // set interval to give time to call stop function
			numOfSingleCallbacks:  10,
			numOfBatchedCallbacks: 375,
			stopManager:           true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// given
			cMq := &mocks.SenderIMock{
				SendFunc:      func(_, _ string, _ *callbacker.Callback) (bool, bool) { return true, false },
				SendBatchFunc: func(_, _ string, _ []*callbacker.Callback) (bool, bool) { return true, false },
			}

			var savedCallbacks []*store.CallbackData
			sMq := &mocks.CallbackerStoreMock{
				SetManyFunc: func(_ context.Context, data []*store.CallbackData) error {
					savedCallbacks = append(savedCallbacks, data...)
					return nil
				},
				SetFunc: func(_ context.Context, data *store.CallbackData) error {
					savedCallbacks = append(savedCallbacks, data)
					return tc.setErr
				},
			}

			sendConfig := &callbacker.SendConfig{
				Delay:                              0,
				PauseAfterSingleModeSuccessfulSend: 0,
				BatchSendInterval:                  time.Millisecond,
			}

			var opts []func(manager *callbacker.SendManager)
			if tc.setEntriesBufferSize > 0 {
				opts = append(opts, callbacker.WithBufferSize(tc.setEntriesBufferSize))
			}

			sut := callbacker.RunNewSendManager("", cMq, sMq, slog.Default(), sendConfig, opts...)

			// add callbacks before starting the manager to queue them
			for range tc.numOfSingleCallbacks {
				sut.Add(&callbacker.CallbackEntry{Data: &callbacker.Callback{}}, false)
			}
			for range tc.numOfBatchedCallbacks {
				sut.Add(&callbacker.CallbackEntry{Data: &callbacker.Callback{}}, true)
			}

			if tc.stopManager {
				sut.GracefulStop()
			} else {
				// give a chance to process
				time.Sleep(200 * time.Millisecond)
			}

			// then
			// check if manager sends callbacks in correct way
			if tc.numOfSingleCallbacks == 0 {
				require.Equal(t, 0, len(cMq.SendCalls()))
			}
			if tc.numOfBatchedCallbacks == 0 {
				require.Equal(t, 0, len(cMq.SendBatchCalls()))
			}

			// check if all callbacks were consumed
			expectedTotal := tc.numOfSingleCallbacks + tc.numOfBatchedCallbacks
			actualBatchedSent := 0
			for _, c := range cMq.SendBatchCalls() {
				actualBatchedSent += len(c.Callbacks)
			}

			actualTotal := len(cMq.SendCalls()) + actualBatchedSent + len(savedCallbacks)

			require.Equal(t, expectedTotal, actualTotal)

			if tc.stopManager {
				// manager should save some callbacks on stoping instead of sending all of them
				require.NotEmpty(t, savedCallbacks)
			} else {
				// if manager was not stopped it should send all callbacks
				require.Empty(t, savedCallbacks)
			}
		})
	}
}

func TestSendManager_FailedCallbacks(t *testing.T) {
	/* Failure scenario
	1. sending failed
	2. put manager in failed state for a specified duration
	3. store all callbacks during the failure period
	4. switch manager to active mode once failure duration is over
	5. send new callbacks again
	*/

	tt := []struct {
		name  string
		batch bool
	}{
		{
			name: "send single callbacks",
		},
		{
			name:  "send batched callbacks",
			batch: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			var sendOK int32
			senderMq := &mocks.SenderIMock{
				SendFunc: func(_, _ string, _ *callbacker.Callback) (bool, bool) {
					return atomic.LoadInt32(&sendOK) == 1, atomic.LoadInt32(&sendOK) != 1
				},
				SendBatchFunc: func(_, _ string, _ []*callbacker.Callback) (bool, bool) {
					return atomic.LoadInt32(&sendOK) == 1, atomic.LoadInt32(&sendOK) != 1
				},
			}

			storeMq := &mocks.CallbackerStoreMock{
				SetFunc: func(_ context.Context, _ *store.CallbackData) error {
					return nil
				},
				SetManyFunc: func(_ context.Context, _ []*store.CallbackData) error {
					return nil
				},
			}

			var preFailureCallbacks []*callbacker.CallbackEntry
			for i := range 10 {
				preFailureCallbacks = append(preFailureCallbacks, &callbacker.CallbackEntry{Data: &callbacker.Callback{TxID: fmt.Sprintf("q %d", i)}})
			}

			var postFailureCallbacks []*callbacker.CallbackEntry
			for i := range 10 {
				postFailureCallbacks = append(postFailureCallbacks, &callbacker.CallbackEntry{Data: &callbacker.Callback{TxID: fmt.Sprintf("a %d", i)}})
			}

			sendConfig := &callbacker.SendConfig{
				Delay:                              0,
				PauseAfterSingleModeSuccessfulSend: 0,
				BatchSendInterval:                  time.Millisecond,
			}

			sut := callbacker.RunNewSendManager("http://unittest.com", senderMq, storeMq, slog.Default(), sendConfig)

			// when
			atomic.StoreInt32(&sendOK, 0) // trigger send failure - this should put the manager in failed state

			// add a few callbacks to send - all should be stored
			for _, c := range preFailureCallbacks {
				sut.Add(c, tc.batch)
			}

			time.Sleep(500 * time.Millisecond) // wait for the failure period to complete

			atomic.StoreInt32(&sendOK, 1) // now all sends should complete successfully
			// add a few callbacks to send - all should be sent
			for _, c := range postFailureCallbacks {
				sut.Add(c, tc.batch)
			}

			// give a chance to process
			time.Sleep(500 * time.Millisecond)

			// then
			// check stored callbacks during failure
			var storedCallbacks []*store.CallbackData
			if tc.batch {
				for _, c := range storeMq.SetManyCalls() {
					storedCallbacks = append(storedCallbacks, c.Data...)
				}
			} else {
				for _, c := range storeMq.SetCalls() {
					storedCallbacks = append(storedCallbacks, c.Dto)
				}
			}

			require.Equal(t, len(preFailureCallbacks), len(storedCallbacks), "all callbacks sent during failure should be stored")
			for _, c := range preFailureCallbacks {
				_, ok := find(storedCallbacks, func(e *store.CallbackData) bool {
					return e.TxID == c.Data.TxID
				})

				require.True(t, ok)
			}

			// check sent callbacks
			var sendCallbacks []*callbacker.Callback
			if tc.batch {
				for _, c := range senderMq.SendBatchCalls() {
					sendCallbacks = append(sendCallbacks, c.Callbacks...)
				}
			} else {
				for _, c := range senderMq.SendCalls() {
					sendCallbacks = append(sendCallbacks, c.Callback)
				}
			}

			require.Equal(t, len(postFailureCallbacks)+len(preFailureCallbacks), len(sendCallbacks), "manager should attempt to send the callback that caused failure (first call) and all callbacks sent after failure")

			_, ok := find(sendCallbacks, func(e *callbacker.Callback) bool {
				return e.TxID == preFailureCallbacks[0].Data.TxID
			})

			require.True(t, ok)

			for _, c := range postFailureCallbacks {
				_, ok := find(sendCallbacks, func(e *callbacker.Callback) bool {
					return e.TxID == c.Data.TxID
				})

				require.True(t, ok)
			}
		})
	}
}

func find[T any](arr []T, predicate func(T) bool) (T, bool) {
	for _, element := range arr {
		if predicate(element) {
			return element, true
		}
	}
	var zero T
	return zero, false
}
