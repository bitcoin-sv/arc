package callbacker_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/callbacker"
	"github.com/bitcoin-sv/arc/internal/callbacker/mocks"
)

func TestCallbackDispatcher(t *testing.T) {
	tcs := []struct {
		name                 string
		sendInterval         time.Duration
		numOfReceivers       int
		numOfSendPerReceiver int
	}{
		{
			name:                 "send",
			sendInterval:         5,
			numOfReceivers:       20,
			numOfSendPerReceiver: 1000,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// given
			cMq := &mocks.SenderIMock{
				SendFunc: func(_, _ string, _ *callbacker.Callback) (bool, bool) { return true, false },
			}

			sut := callbacker.NewCallbackDispatcher(cMq, func(_ string) callbacker.SendManagerI {
				return &mocks.SendManagerIMock{
					EnqueueFunc:      func(_ callbacker.CallbackEntry) {},
					GracefulStopFunc: func() {},
				}
			})

			var receivers []string
			for i := range tc.numOfReceivers {
				receivers = append(receivers, fmt.Sprintf("url_%d", i))
			}

			// when

			for _, receiver := range receivers {
				sut.Dispatch(receiver, &callbacker.CallbackEntry{Token: "", Data: &callbacker.Callback{}})
			}
			// send callbacks to receiver
			sut.GracefulStop()

			// then
			require.Equal(t, tc.numOfReceivers, sut.GetLenManagers())
		})
	}
}
