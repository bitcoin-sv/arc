package k8s_watcher_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

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
	}{
		{
			name: "unlock records for metamorph-pod-2",
			podNames: []map[string]struct{}{
				{"metamorph-pod-1": {}, "metamorph-pod-2": {}, "api-pod-1": {}, "blocktx-pod-1": {}},
				{"metamorph-pod-1": {}, "blocktx-pod-1": {}},
				{"metamorph-pod-1": {}, "metamorph-pod-3": {}, "api-pod-2": {}, "blocktx-pod-1": {}},
			},

			expectedMetamorphSetUnlockedByNameCalls: 1,
		},
		{
			name:           "error - get pod names",
			podNames:       []map[string]struct{}{{"": {}}},
			getPodNamesErr: errors.New("failed to get pod names"),

			expectedMetamorphSetUnlockedByNameCalls: 0,
		},
		{
			name: "error - set unlocked",
			podNames: []map[string]struct{}{
				{"metamorph-pod-1": {}, "metamorph-pod-2": {}},
				{"metamorph-pod-1": {}},
				{"metamorph-pod-1": {}, "metamorph-pod-3": {}},
			},
			setUnlockedErr: errors.New("failed to set unlocked"),

			expectedMetamorphSetUnlockedByNameCalls: 5,
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
			k8sClientMock := &mocks.K8sClientMock{
				GetRunningPodNamesFunc: func(_ context.Context, _ string, _ string) (map[string]struct{}, error) {
					if getPodNamesErrTest != nil {
						return nil, getPodNamesErrTest
					}

					podNames := podNamestTest[iteration]

					iteration++

					return podNames, nil
				},
			}

			tickerChannel := make(chan time.Time, 1)

			ticker := &mocks.TickerMock{
				TickFunc: func() <-chan time.Time {
					return tickerChannel
				},
				StopFunc: func() {},
			}

			watcher := k8s_watcher.New(metamorphMock, k8sClientMock, "test-namespace", k8s_watcher.WithMetamorphTicker(ticker),
				k8s_watcher.WithLogger(slog.Default()),
				k8s_watcher.WithRetryInterval(20*time.Millisecond),
			)
			err := watcher.Start()
			require.NoError(t, err)

			for range tc.podNames {
				tickerChannel <- time.Now()
			}

			watcher.Shutdown()

			require.Equal(t, tc.expectedMetamorphSetUnlockedByNameCalls, len(metamorphMock.SetUnlockedByNameCalls()))
		})
	}
}
