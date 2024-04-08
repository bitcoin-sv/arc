package k8s_watcher_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/k8s_watcher"
	"github.com/bitcoin-sv/arc/internal/k8s_watcher/mock"
	"github.com/lmittmann/tint"
	"github.com/stretchr/testify/require"
)

//go:generate moq -pkg mock -out ./mock/metamorph_client_mock.go ../../pkg/metamorph TransactionMaintainer
//go:generate moq -pkg mock -out ./mock/blocktx_client_mock.go ../../pkg/blocktx BlocktxClient
//go:generate moq -pkg mock -out ./mock/k8s_client_client_mock.go . K8sClient
//go:generate moq -pkg mock -out ./mock/ticker_mock.go . Ticker

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

			expectedMetamorphSetUnlockedByNameCalls: 1,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			metamorphMock := &mock.TransactionMaintainerMock{
				SetUnlockedByNameFunc: func(ctx context.Context, name string) (int64, error) {
					require.Equal(t, "metamorph-pod-2", name)

					if tc.setUnlockedErr != nil {
						return 0, tc.setUnlockedErr
					}

					return 3, nil
				},
			}
			blocktxMock := &mock.BlocktxClientMock{}

			iteration := 0
			k8sClientMock := &mock.K8sClientMock{
				GetRunningPodNamesFunc: func(ctx context.Context, namespace string, service string) (map[string]struct{}, error) {
					if tc.getPodNamesErr != nil {
						return nil, tc.getPodNamesErr
					}

					podNames := tc.podNames[iteration]

					iteration++

					return podNames, nil
				},
			}

			tickerChannel := make(chan time.Time, 1)

			ticker := &mock.TickerMock{
				TickFunc: func() <-chan time.Time {
					return tickerChannel
				},
				StopFunc: func() {},
			}

			watcher := k8s_watcher.New(metamorphMock, blocktxMock, k8sClientMock, "test-namespace", k8s_watcher.WithMetamorphTicker(ticker),
				k8s_watcher.WithLogger(slog.New(tint.NewHandler(os.Stdout, &tint.Options{Level: slog.LevelInfo, TimeFormat: time.Kitchen}))),
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

func TestStartBlocktxWatcher(t *testing.T) {
	tt := []struct {
		name           string
		podNames       []map[string]struct{}
		getPodNamesErr error
		setUnlockedErr error

		expectedBlocktxDelUnfinishedBlockProcessingFunc int
	}{
		{
			name: "unlock records for metamorph-pod-2",
			podNames: []map[string]struct{}{
				{"blocktx-pod-1": {}, "blocktx-pod-2": {}, "api-pod-1": {}, "metamorph-pod-1": {}},
				{"blocktx-pod-1": {}, "metamorph-pod-1": {}},
				{"blocktx-pod-1": {}, "blocktx-pod-3": {}, "api-pod-2": {}, "metamorph-pod-1": {}},
			},

			expectedBlocktxDelUnfinishedBlockProcessingFunc: 1,
		},
		{
			name:           "error - get pod names",
			podNames:       []map[string]struct{}{{"": {}}},
			getPodNamesErr: errors.New("failed to get pod names"),

			expectedBlocktxDelUnfinishedBlockProcessingFunc: 0,
		},
		{
			name: "error - set unlocked",
			podNames: []map[string]struct{}{
				{"blocktx-pod-1": {}, "blocktx-pod-2": {}},
				{"blocktx-pod-1": {}},
				{"blocktx-pod-1": {}, "blocktx-pod-3": {}},
			},
			setUnlockedErr: errors.New("failed to set unlocked"),

			expectedBlocktxDelUnfinishedBlockProcessingFunc: 1,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			metamorphMock := &mock.TransactionMaintainerMock{}
			blocktxMock := &mock.BlocktxClientMock{
				DelUnfinishedBlockProcessingFunc: func(ctx context.Context, processedBy string) error {
					return nil
				},
			}

			iteration := 0
			k8sClientMock := &mock.K8sClientMock{
				GetRunningPodNamesFunc: func(ctx context.Context, namespace string, service string) (map[string]struct{}, error) {
					if tc.getPodNamesErr != nil {
						return nil, tc.getPodNamesErr
					}

					podNames := tc.podNames[iteration]

					iteration++

					return podNames, nil
				},
			}

			tickerChannel := make(chan time.Time, 1)

			ticker := &mock.TickerMock{
				TickFunc: func() <-chan time.Time {
					return tickerChannel
				},
				StopFunc: func() {},
			}

			watcher := k8s_watcher.New(metamorphMock, blocktxMock, k8sClientMock, "test-namespace", k8s_watcher.WithBlocktxTicker(ticker),
				k8s_watcher.WithLogger(slog.New(tint.NewHandler(os.Stdout, &tint.Options{Level: slog.LevelInfo, TimeFormat: time.Kitchen}))),
			)
			err := watcher.Start()
			require.NoError(t, err)

			for range tc.podNames {
				tickerChannel <- time.Now()
			}

			watcher.Shutdown()

			require.Equal(t, tc.expectedBlocktxDelUnfinishedBlockProcessingFunc, len(blocktxMock.DelUnfinishedBlockProcessingCalls()))
		})
	}
}
