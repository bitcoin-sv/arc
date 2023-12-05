package k8s_watcher_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/k8s_watcher"
	"github.com/bitcoin-sv/arc/k8s_watcher/mock"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/lmittmann/tint"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

//go:generate moq -pkg mock -out ./mock/metamorph_api_client_mock.go ../metamorph/metamorph_api MetaMorphAPIClient
//go:generate moq -pkg mock -out ./mock/k8s_client_client_mock.go . K8sClient
//go:generate moq -pkg mock -out ./mock/ticker_mock.go . Ticker

func TestStart(t *testing.T) {
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
			metamorphMock := &mock.MetaMorphAPIClientMock{
				SetUnlockedByNameFunc: func(ctx context.Context, in *metamorph_api.SetUnlockedByNameRequest, opts ...grpc.CallOption) (*metamorph_api.SetUnlockedByNameResponse, error) {
					require.Equal(t, "metamorph-pod-2", in.Name)

					if tc.setUnlockedErr != nil {
						return nil, tc.setUnlockedErr
					}

					return &metamorph_api.SetUnlockedByNameResponse{RecordsAffected: 3}, nil
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

			watcher := k8s_watcher.New(metamorphMock, k8sClientMock, "test-namespace", k8s_watcher.WithTicker(ticker),
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
