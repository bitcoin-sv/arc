package k8s_watcher_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	cbcMocks "github.com/bitcoin-sv/arc/internal/callbacker/mocks"
	"github.com/bitcoin-sv/arc/internal/k8s_watcher"
	"github.com/bitcoin-sv/arc/internal/k8s_watcher/mocks"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	mtmMocks "github.com/bitcoin-sv/arc/internal/metamorph/mocks"
)

func TestStartWatcher(t *testing.T) {
	tt := []struct {
		name           string
		podNames       []string
		getPodNamesErr error

		expectedMetamorphUpdateInstancesCalls int
		expectedGetRunningPodsCalls           int
	}{
		{
			name:     "success",
			podNames: []string{"metamorph-pod-1", "metamorph-pod-2"},

			expectedMetamorphUpdateInstancesCalls: 5,
			expectedGetRunningPodsCalls:           10,
		},
		{
			name:           "error - get pod names",
			podNames:       []string{""},
			getPodNamesErr: errors.New("failed to get pod names"),

			expectedMetamorphUpdateInstancesCalls: 0,
			expectedGetRunningPodsCalls:           10,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			k8sClientMock := &mocks.K8sClientMock{
				GetRunningPodNamesSliceFunc: func(_ context.Context, _ string, _ string) ([]string, error) {
					return tc.podNames, tc.getPodNamesErr
				},
			}
			metamorphMock := &mtmMocks.MetaMorphAPIClientMock{
				UpdateInstancesFunc: func(_ context.Context, _ *metamorph_api.UpdateInstancesRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
					return &emptypb.Empty{}, nil
				},
			}
			callbackerClient := &cbcMocks.CallbackerAPIClientMock{}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

			watcher := k8s_watcher.New(logger, metamorphMock, callbackerClient, k8sClientMock, "test-namespace", k8s_watcher.WithUpdateInterval(20*time.Millisecond))
			err := watcher.Start()
			require.NoError(t, err)

			time.Sleep(110 * time.Millisecond)
			watcher.Shutdown()

			require.Equal(t, tc.expectedMetamorphUpdateInstancesCalls, len(metamorphMock.UpdateInstancesCalls()))
			require.Equal(t, tc.expectedGetRunningPodsCalls, len(k8sClientMock.GetRunningPodNamesSliceCalls()))
		})
	}
}
