package callbacker

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_CallbackDispatcher(t *testing.T) {
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
			sendInterval:         time.Nanosecond, // set interval to give time to call stop function
			numOfReceivers:       100,
			numOfSendPerReceiver: 200,
			stopDispatcher:       true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// given
			cMq := &CallbackerIMock{
				SendFunc: func(url, token string, callback *Callback) {},
			}

			sut := NewCallbackDispatcher(cMq, tc.sendInterval)

			var receivers []string
			for i := range tc.numOfReceivers {
				receivers = append(receivers, fmt.Sprintf("url_%d", i))
			}

			// when
			// send callbacks to receiver
			wg := &sync.WaitGroup{}
			for range tc.numOfSendPerReceiver {
				wg.Add(1)
				go func() {
					for _, url := range receivers {
						sut.Send(url, "", nil)
					}
					wg.Done()
				}()
			}
			wg.Wait()

			if tc.stopDispatcher {
				sut.GracefulStop()
			} else {
				// give a chance to process
				time.Sleep(5 * time.Millisecond)
			}

			// then
			require.Equal(t, tc.numOfReceivers, len(sut.managers))
			require.Equal(t, tc.numOfReceivers*tc.numOfSendPerReceiver, len(cMq.SendCalls()))
		})
	}
}

func Test_sendManager(t *testing.T) {
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
			name:         "send callbacks on stopping",
			sendInterval: time.Millisecond, // set interval to give time to call stop function
			numOfSends:   10,
			stopManager:  true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// given
			cMq := &CallbackerIMock{
				SendFunc: func(url, token string, callback *Callback) {},
			}

			sut := &sendManager{
				url:   "",
				c:     cMq,
				sleep: tc.sendInterval,

				ch: make(chan *callbackEntry),
			}

			// add callbacks before starting the manager to queue them
			for range tc.numOfSends {
				sut.Add("", nil)
			}

			// when
			sut.run()

			if tc.stopManager {
				sut.GracefulStop()
			} else {
				// give a chance to process
				time.Sleep(5 * time.Millisecond)
			}

			// then
			require.Len(t, cMq.SendCalls(), tc.numOfSends)
		})
	}
}
