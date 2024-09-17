package callbacker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/callbacker/store"
	"github.com/bitcoin-sv/arc/internal/callbacker/store/mocks"
	"github.com/stretchr/testify/require"
)

func TestCallbackDispatcher(t *testing.T) {
	tcs := []struct {
		name                 string
		sendInterval         time.Duration
		numOfReceivers       int
		numOfSendPerReceiver int
		stopDispatcher       bool
	}{
		{
			name:                 "send",
			sendInterval:         0,
			numOfReceivers:       20,
			numOfSendPerReceiver: 1000,
		},
		{
			name:                 "process callbacks on stopping",
			sendInterval:         5 * time.Millisecond, // set interval to give time to call stop function
			numOfReceivers:       100,
			numOfSendPerReceiver: 200,
			stopDispatcher:       true,
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

			sut := NewCallbackDispatcher(cMq, sMq, slog.Default(), tc.sendInterval, 0, 0)

			var receivers []string
			for i := range tc.numOfReceivers {
				receivers = append(receivers, fmt.Sprintf("url_%d", i))
			}

			// when
			// send callbacks to receiver
			wg := &sync.WaitGroup{}
			wg.Add(tc.numOfSendPerReceiver)
			for range tc.numOfSendPerReceiver {
				go func() {
					for _, url := range receivers {
						sut.Send(url, "", &Callback{})
					}
					wg.Done()
				}()
			}
			wg.Wait()

			if tc.stopDispatcher {
				sut.GracefulStop()
			} else {
				// give a chance to process
				time.Sleep(100 * time.Millisecond)
			}

			// then
			require.Equal(t, tc.numOfReceivers, len(sut.managers))
			if tc.stopDispatcher {
				require.NotEmpty(t, savedCallbacks)
				require.Equal(t, tc.numOfReceivers*tc.numOfSendPerReceiver, len(cMq.SendCalls())+len(savedCallbacks))
			} else {
				require.Empty(t, savedCallbacks)
				require.Equal(t, tc.numOfReceivers*tc.numOfSendPerReceiver, len(cMq.SendCalls()))
			}
		})
	}
}
