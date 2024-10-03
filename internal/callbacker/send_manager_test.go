package callbacker

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/callbacker/store"
	"github.com/bitcoin-sv/arc/internal/callbacker/store/mocks"
	"github.com/stretchr/testify/require"
)

func TestSendManager(t *testing.T) {
	tcs := []struct {
		name                  string
		singleSendInterval    time.Duration
		numOfSingleCallbacks  int
		numOfBatchedCallbacks int
		stopManager           bool
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
			cMq := &CallbackerIMock{
				SendFunc:      func(_, _ string, _ *Callback) bool { return true },
				SendBatchFunc: func(_, _ string, _ []*Callback) bool { return true },
			}

			var savedCallbacks []*store.CallbackData
			sMq := &mocks.CallbackerStoreMock{
				SetManyFunc: func(_ context.Context, data []*store.CallbackData) error {
					savedCallbacks = append(savedCallbacks, data...)
					return nil
				},
			}

			sut := &sendManager{
				url:               "",
				c:                 cMq,
				s:                 sMq,
				singleSendSleep:   tc.singleSendInterval,
				batchSendInterval: 5 * time.Millisecond,

				entries:      make(chan *CallbackEntry),
				batchEntries: make(chan *CallbackEntry),
				stop:         make(chan struct{}),
			}

			// add callbacks before starting the manager to queue them
			for range tc.numOfSingleCallbacks {
				sut.Add(&CallbackEntry{Data: &Callback{}}, false)
			}
			for range tc.numOfBatchedCallbacks {
				sut.Add(&CallbackEntry{Data: &Callback{}}, true)
			}

			// when
			sut.run()

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

func TestSendManager_Quarantine(t *testing.T) {
	/* Quarantine scenario
	1. sending failed
	2. put manager in quarantine for a specified duration
	3. store all callbacks during the quarantine period
	4. switch manager to active mode once quarantine is over
	5. send new callbacks after quarantine
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
			sendOK := true
			senderMq := &CallbackerIMock{
				SendFunc:      func(_, _ string, _ *Callback) bool { return sendOK },
				SendBatchFunc: func(_, _ string, _ []*Callback) bool { return sendOK },
			}

			storeMq := &mocks.CallbackerStoreMock{
				SetFunc: func(_ context.Context, _ *store.CallbackData) error {
					return nil
				},
				SetManyFunc: func(_ context.Context, _ []*store.CallbackData) error {
					return nil
				},
			}

			policy := quarantinePolicy{
				baseDuration:        200 * time.Millisecond,
				permQuarantineAfter: time.Hour,
				now:                 time.Now,
			}

			var preQuarantineCallbacks []*CallbackEntry
			for i := range 10 {
				preQuarantineCallbacks = append(preQuarantineCallbacks, &CallbackEntry{Data: &Callback{TxID: fmt.Sprintf("q %d", i)}})
			}

			var postQuarantineCallbacks []*CallbackEntry
			for i := range 10 {
				postQuarantineCallbacks = append(postQuarantineCallbacks, &CallbackEntry{Data: &Callback{TxID: fmt.Sprintf("a %d", i)}})
			}

			sut := runNewSendManager("http://unittest.com", senderMq, storeMq, slog.Default(), &policy, 0, time.Millisecond)

			// when
			sendOK = false // trigger send failure - this should put the manager in quarantine

			// add a few callbacks to send - all should be stored
			for _, c := range preQuarantineCallbacks {
				sut.Add(c, tc.batch)
				// wait to make sure first callback is processed as first
				time.Sleep(10 * time.Millisecond)
			}
			require.Equal(t, QuarantineMode, sut.getMode())

			time.Sleep(policy.baseDuration + 20*time.Millisecond) // wait for the quarantine period to complete
			require.Equal(t, ActiveMode, sut.getMode())

			sendOK = true // now all sends should complete successfully
			// add a few callbacks to send - all should be sent
			for _, c := range postQuarantineCallbacks {
				sut.Add(c, tc.batch)
			}

			// give a chance to process
			time.Sleep(50 * time.Millisecond)

			// then
			// check stored callbacks during quarantine
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

			require.Equal(t, len(preQuarantineCallbacks), len(storedCallbacks), "all callbacks sent during quarantine should be stored")
			for _, c := range preQuarantineCallbacks {
				_, ok := find(storedCallbacks, func(e *store.CallbackData) bool {
					return e.TxID == c.Data.TxID
				})

				require.True(t, ok)
			}

			// check sent callbacks
			var sendCallbacks []*Callback
			if tc.batch {
				for _, c := range senderMq.SendBatchCalls() {
					sendCallbacks = append(sendCallbacks, c.Callbacks...)
				}
			} else {
				for _, c := range senderMq.SendCalls() {
					sendCallbacks = append(sendCallbacks, c.Callback)
				}
			}

			require.Equal(t, len(postQuarantineCallbacks)+1, len(sendCallbacks), "manager should attempt to send the callback that caused quarantine (first call) and all callbacks sent after quarantine")

			_, ok := find(sendCallbacks, func(e *Callback) bool {
				return e.TxID == preQuarantineCallbacks[0].Data.TxID
			})

			require.True(t, ok)

			for _, c := range postQuarantineCallbacks {
				_, ok := find(sendCallbacks, func(e *Callback) bool {
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
