package callbacker

import (
	"context"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/callbacker/store"
	"github.com/bitcoin-sv/arc/internal/callbacker/store/mocks"
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

			var savedCallbacks []*store.CallbackData
			sMq := &mocks.CallbackerStoreMock{
				SetManyFunc: func(ctx context.Context, data []*store.CallbackData) error {
					savedCallbacks = append(savedCallbacks, data...)
					return nil
				},
			}

			sut := NewCallbackDispatcher(cMq, sMq, tc.sendInterval)

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
				time.Sleep(50 * time.Millisecond)
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

func Test_CallbackDispatcher_Init(t *testing.T) {
	tcs := []struct {
		name                 string
		danglingCallbacksNum int
	}{
		{
			name:                 "no dangling callbacks",
			danglingCallbacksNum: 0,
		},
		{
			name:                 "callbacks to process on init",
			danglingCallbacksNum: 259,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// given
			var danglingCallbacks []*store.CallbackData
			for range tc.danglingCallbacksNum {
				danglingCallbacks = append(danglingCallbacks, &store.CallbackData{})
			}

			cMq := &CallbackerIMock{
				SendFunc: func(url, token string, callback *Callback) {},
			}

			sMq := &mocks.CallbackerStoreMock{
				PopManyFunc: func(ctx context.Context, limit int) ([]*store.CallbackData, error) {
					limit = int(math.Min(float64(len(danglingCallbacks)), float64(limit)))

					r := danglingCallbacks[:limit]
					danglingCallbacks = danglingCallbacks[limit:]

					return r, nil
				},
			}

			sut := NewCallbackDispatcher(cMq, sMq, 0)

			// when
			err := sut.Init()
			time.Sleep(50 * time.Millisecond)

			// then
			require.NoError(t, err)
			require.Equal(t, tc.danglingCallbacksNum, len(cMq.SendCalls()))
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

				entries: make(chan *callbackEntry),
				stop:    make(chan struct{}),
			}

			// add callbacks before starting the manager to queue them
			for range tc.numOfSends {
				sut.Add("", &Callback{})
			}

			// when
			sut.run()

			if tc.stopManager {
				sut.GracefulStop()
			} else {
				// give a chance to process or save on quit
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
