package k8s_watcher_test

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
	cbcMocks "github.com/bitcoin-sv/arc/internal/callbacker/mocks"
	"github.com/bitcoin-sv/arc/internal/k8s_watcher"
	"github.com/bitcoin-sv/arc/internal/k8s_watcher/mocks"
	mtmMocks "github.com/bitcoin-sv/arc/internal/metamorph/mocks"
)

func TestStartMetamorphWatcher(t *testing.T) {
	tt := []struct {
		name           string
		podNames       []map[string]struct{}
		getPodNamesErr error
		setUnlockedErr error

		expectedMetamorphSetUnlockedByNameCalls int
		expectedCallbackerUnmappingCalls        int
	}{
		{
			name: "check releasing pod resources",
			podNames: []map[string]struct{}{
				{"metamorph-pod-1": {}, "metamorph-pod-2": {}, "callbacker-pod-1": {}, "callbacker-pod-2": {}, "api-pod-1": {}, "blocktx-pod-1": {}},
				{"metamorph-pod-1": {}, "metamorph-pod-2": {}, "callbacker-pod-1": {}, "callbacker-pod-2": {}, "api-pod-1": {}, "blocktx-pod-1": {}},
				{"metamorph-pod-1": {}, "callbacker-pod-1": {}, "blocktx-pod-1": {}},
				{"metamorph-pod-1": {}, "callbacker-pod-1": {}, "blocktx-pod-1": {}},
				{"metamorph-pod-1": {}, "metamorph-pod-3": {}, "callbacker-pod-1": {}, "callbacker-pod-3": {}, "api-pod-2": {}, "blocktx-pod-1": {}},
				{"metamorph-pod-1": {}, "metamorph-pod-3": {}, "callbacker-pod-1": {}, "callbacker-pod-3": {}, "api-pod-2": {}, "blocktx-pod-1": {}},
			},

			expectedMetamorphSetUnlockedByNameCalls: 1,
			expectedCallbackerUnmappingCalls:        1,
		},
		{
			name:           "error - get pod names",
			podNames:       []map[string]struct{}{{"": {}}},
			getPodNamesErr: errors.New("failed to get pod names"),

			expectedMetamorphSetUnlockedByNameCalls: 0,
		},
		{
			name: "check releasing pod resources with error/retries",
			podNames: []map[string]struct{}{
				{"metamorph-pod-1": {}, "metamorph-pod-2": {}, "callbacker-pod-1": {}, "callbacker-pod-2": {}},
				{"metamorph-pod-1": {}, "metamorph-pod-2": {}, "callbacker-pod-1": {}, "callbacker-pod-2": {}},
				{"metamorph-pod-1": {}, "callbacker-pod-1": {}},
				{"metamorph-pod-1": {}, "callbacker-pod-1": {}},
				{"metamorph-pod-1": {}, "metamorph-pod-3": {}, "callbacker-pod-1": {}, "callbacker-pod-3": {}},
				{"metamorph-pod-1": {}, "metamorph-pod-3": {}, "callbacker-pod-1": {}, "callbacker-pod-3": {}},
			},
			setUnlockedErr: errors.New("failed"),

			expectedMetamorphSetUnlockedByNameCalls: 5,
			expectedCallbackerUnmappingCalls:        5,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			setUnlockedErrTest := tc.setUnlockedErr
			metamorphMock := &mtmMocks.TransactionMaintainerMock{
				SetUnlockedByNameFunc: func(_ context.Context, name string) (int64, error) {
					require.Equal(t, "metamorph-pod-2", name)

					if setUnlockedErrTest != nil {
						return 0, setUnlockedErrTest
					}

					return 3, nil
				},
			}
			iteration := 0
			getPodNamesErrTest := tc.getPodNamesErr
			podNamestTest := tc.podNames
			var m sync.Mutex
			k8sClientMock := &mocks.K8sClientMock{
				GetRunningPodNamesFunc: func(_ context.Context, _ string, _ string) (map[string]struct{}, error) {
					m.Lock()
					defer m.Unlock()
					if getPodNamesErrTest != nil {
						return nil, getPodNamesErrTest
					}

					podNames := podNamestTest[iteration]

					iteration++

					return podNames, nil
				},
			}
			callbackerClient := &cbcMocks.CallbackerAPIClientMock{
				DeleteURLMappingFunc: func(_ context.Context, _ *callbacker_api.DeleteURLMappingRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
					if setUnlockedErrTest != nil {
						return nil, setUnlockedErrTest
					}
					return nil, nil
				},
			}

			tickerChannel := make(chan time.Time, 1)
			ticker := &mocks.TickerMock{
				TickFunc: func() <-chan time.Time {
					return tickerChannel
				},
				StopFunc: func() {},
			}

			tickerChannel2 := make(chan time.Time, 1)
			ticker2 := &mocks.TickerMock{
				TickFunc: func() <-chan time.Time {
					return tickerChannel2
				},
				StopFunc: func() {},
			}

			watcher := k8s_watcher.New(metamorphMock, callbackerClient, k8sClientMock, "test-namespace", k8s_watcher.WithMetamorphTicker(ticker), k8s_watcher.WithCallbackerTicker(ticker2),
				k8s_watcher.WithLogger(slog.Default()),
				k8s_watcher.WithRetryInterval(20*time.Millisecond),
			)
			err := watcher.Start()
			require.NoError(t, err)

			for range 3 {
				tickerChannel <- time.Now()
				tickerChannel2 <- time.Now()
				time.Sleep(50 * time.Millisecond)
			}

			time.Sleep(100 * time.Millisecond)
			watcher.Shutdown()

			require.Equal(t, tc.expectedMetamorphSetUnlockedByNameCalls, len(metamorphMock.SetUnlockedByNameCalls()))
			require.Equal(t, tc.expectedCallbackerUnmappingCalls, len(callbackerClient.DeleteURLMappingCalls()))
		})
	}
}
