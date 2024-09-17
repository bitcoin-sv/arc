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
		name         string
		sendInterval time.Duration
		numOfSends   int
		stopManager  bool
	}{
		{
			name:         "send callbacks when run",
			sendInterval: 0,
			numOfSends:   100,
		},
		{
			name:         "save callbacks on stopping",
			sendInterval: time.Millisecond, // set interval to give time to call stop function
			numOfSends:   10,
			stopManager:  true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// given
			cMq := &CallbackerIMock{
				SendFunc: func(url, token string, callback *Callback) bool { return true },
			}
			var savedCallbacks []*store.CallbackData
			sMq := &mocks.CallbackerStoreMock{
				SetManyFunc: func(ctx context.Context, data []*store.CallbackData) error {
					savedCallbacks = append(savedCallbacks, data...)
					return nil
				},
			}

			sut := &sendManager{
				url:   "",
				c:     cMq,
				s:     sMq,
				sleep: tc.sendInterval,

				entries: make(chan *CallbackEntry),
				stop:    make(chan struct{}),
			}

			// add callbacks before starting the manager to queue them
			for range tc.numOfSends {
				sut.Add(&CallbackEntry{Data: &Callback{}})
			}

			// when
			sut.run()

			if tc.stopManager {
				sut.GracefulStop()
			} else {
				// give a chance to process
				time.Sleep(50 * time.Millisecond)
			}

			// then
			if tc.stopManager {
				require.NotEmpty(t, savedCallbacks)
				require.Equal(t, tc.numOfSends, len(cMq.SendCalls())+len(savedCallbacks))
			} else {
				require.Empty(t, savedCallbacks)
				require.Equal(t, tc.numOfSends, len(cMq.SendCalls()))
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

	// given
	sendOK := true
	senderMq := &CallbackerIMock{
		SendFunc: func(url, token string, callback *Callback) bool { return sendOK },
	}

	storeMq := &mocks.CallbackerStoreMock{
		SetFunc: func(ctx context.Context, data *store.CallbackData) error {
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
		preQuarantineCallbacks = append(preQuarantineCallbacks, &CallbackEntry{Token: fmt.Sprintf("q %d", i), Data: &Callback{}})
	}

	var postQuarantineCallbacks []*CallbackEntry
	for i := range 10 {
		postQuarantineCallbacks = append(postQuarantineCallbacks, &CallbackEntry{Token: fmt.Sprintf("a %d", i), Data: &Callback{}})
	}

	sut := runNewSendManager("http://unittest.com", senderMq, storeMq, slog.Default(), 0, &policy)

	// when
	sendOK = false // trigger send failure - this should put the manager in quarantine

	// add a few callbacks to send - all should be stored
	for _, c := range preQuarantineCallbacks {
		sut.Add(c)
		time.Sleep(10 * time.Millisecond)
	}
	require.Equal(t, QuarantineMode, sut.getMode())

	time.Sleep(policy.baseDuration + 20*time.Millisecond) // wait for the quarantine period to complete
	require.Equal(t, ActiveMode, sut.getMode())

	sendOK = true // now all sends should complete successfully
	// add a few callbacks to send - all should be sent
	for _, c := range postQuarantineCallbacks {
		sut.Add(c)
	}

	// give a chance to process
	time.Sleep(50 * time.Millisecond)

	// then
	storedCallbacks := storeMq.SetCalls()
	require.Equal(t, len(preQuarantineCallbacks), len(storedCallbacks), "all callbacks sent during quarantine should be stored")
	for _, c := range preQuarantineCallbacks {
		_, ok := find(storedCallbacks, func(e struct {
			Ctx context.Context
			Dto *store.CallbackData
		}) bool {
			return e.Dto.Token == c.Token
		})

		require.True(t, ok)
	}

	sendCallbacks := senderMq.SendCalls()
	require.Equal(t, len(postQuarantineCallbacks)+1, len(sendCallbacks), "manager should attempt to resend the callback that caused quarantine and all callbacks sent after quarantine")

	_, ok := find(sendCallbacks, func(e struct {
		URL      string
		Token    string
		Callback *Callback
	}) bool {
		return e.Token == preQuarantineCallbacks[0].Token
	})

	require.True(t, ok)
	for _, c := range postQuarantineCallbacks {
		_, ok := find(sendCallbacks, func(e struct {
			URL      string
			Token    string
			Callback *Callback
		}) bool {
			return e.Token == c.Token
		})

		require.True(t, ok)
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
