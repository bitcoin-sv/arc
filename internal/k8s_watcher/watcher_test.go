package k8s_watcher_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
	cbcMocks "github.com/bitcoin-sv/arc/internal/callbacker/mocks"
	"github.com/bitcoin-sv/arc/internal/k8s_watcher"
	"github.com/bitcoin-sv/arc/internal/k8s_watcher/mocks"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
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
			metamorphMock := &mtmMocks.MetaMorphAPIClientMock{
				UpdateInstancesFunc: func(_ context.Context, _ *metamorph_api.UpdateInstancesRequest, _ ...grpc.CallOption) (*metamorph_api.UpdateInstancesResponse, error) {
					return &metamorph_api.UpdateInstancesResponse{Response: "response"}, nil
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
				UpdateInstancesFunc: func(_ context.Context, _ *callbacker_api.UpdateInstancesRequest, _ ...grpc.CallOption) (*callbacker_api.UpdateInstancesResponse, error) {
					if setUnlockedErrTest != nil {
						return nil, setUnlockedErrTest
					}
					return &callbacker_api.UpdateInstancesResponse{Response: "response"}, nil
				},
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

			watcher := k8s_watcher.New(logger, metamorphMock, callbackerClient, k8sClientMock, "test-namespace")
			err := watcher.Start()
			require.NoError(t, err)

			time.Sleep(100 * time.Millisecond)
			watcher.Shutdown()

			require.Equal(t, tc.expectedMetamorphSetUnlockedByNameCalls, len(metamorphMock.UpdateInstancesCalls()))
			require.Equal(t, tc.expectedCallbackerUnmappingCalls, len(callbackerClient.UpdateInstancesCalls()))
		})
	}
}
