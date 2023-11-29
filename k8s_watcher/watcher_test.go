package k8s_watcher_test

import (
	"context"
	"errors"
	"k8s.io/apimachinery/pkg/runtime"
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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

//go:generate moq -pkg mock -out ./mock/metamorph_api_client_mock.go ../metamorph/metamorph_api MetaMorphAPIClient
//go:generate moq -pkg mock -out ./mock/k8s_client_client_mock.go . K8sClient

func TestStart(t *testing.T) {
	tt := []struct {
		name           string
		event          runtime.Object
		setUnlockedErr error
		getPodWatcher  error

		expectedErrorStr                        string
		expectedMetamorphSetUnlockedByNameCalls int
	}{
		{
			name:  "unlock records for metamorph pod",
			event: &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "metamorph-685fb9d6b4-tlt86"}},

			expectedMetamorphSetUnlockedByNameCalls: 1,
		},
		{
			name:           "set unlocked - error",
			event:          &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "metamorph-685fb9d6b4-tlt86"}},
			setUnlockedErr: errors.New("failed to set unlocked"),

			expectedMetamorphSetUnlockedByNameCalls: 1,
		},
		{
			name:  "event is not a pod",
			event: &v1.Node{},

			expectedMetamorphSetUnlockedByNameCalls: 0,
		},
		{
			name:          "get pod watcher - error",
			event:         &v1.Node{},
			getPodWatcher: errors.New("failed to get pod watcher"),

			expectedErrorStr:                        "failed to get pod watcher",
			expectedMetamorphSetUnlockedByNameCalls: 0,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			metamorphMock := &mock.MetaMorphAPIClientMock{
				SetUnlockedByNameFunc: func(ctx context.Context, in *metamorph_api.SetUnlockedByNameRequest, opts ...grpc.CallOption) (*metamorph_api.SetUnlockedByNameResponse, error) {
					require.Equal(t, "metamorph-685fb9d6b4-tlt86", in.Name)

					if tc.setUnlockedErr != nil {
						return nil, tc.setUnlockedErr
					}

					return &metamorph_api.SetUnlockedByNameResponse{RecordsAffected: 3}, nil
				},
			}

			watcherMock := watch.NewFake()
			k8sClientMock := &mock.K8sClientMock{
				GetPodWatcherFunc: func(ctx context.Context, namespace string, podName string) (watch.Interface, error) {
					return watcherMock, tc.getPodWatcher
				},
			}

			watcher := k8s_watcher.New(metamorphMock, k8sClientMock, "test-namespace",
				k8s_watcher.WithLogger(slog.New(tint.NewHandler(os.Stdout, &tint.Options{Level: slog.LevelInfo, TimeFormat: time.Kitchen}))),
			)
			err := watcher.Start()
			if tc.expectedErrorStr != "" || err != nil {
				require.ErrorContains(t, err, tc.expectedErrorStr)
				return
			} else {
				require.NoError(t, err)
			}

			watcherMock.Delete(tc.event)
			watcher.Shutdown()

			require.Equal(t, tc.expectedMetamorphSetUnlockedByNameCalls, len(metamorphMock.SetUnlockedByNameCalls()))
		})
	}
}
