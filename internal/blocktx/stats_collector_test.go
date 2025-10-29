package blocktx_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx"
	testutils "github.com/bitcoin-sv/arc/pkg/test_utils"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

	blocktxmocks "github.com/bitcoin-sv/arc/internal/blocktx/mocks"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/bitcoin-sv/arc/internal/blocktx/store/mocks"
	"github.com/bitcoin-sv/arc/internal/p2p"
	p2p_mocks "github.com/bitcoin-sv/arc/internal/p2p/mocks"
)

func TestStatsCollector_Start(t *testing.T) {
	tt := []struct {
		name        string
		getStatsErr error

		expectedBlockGaps float64
		connectedPeers    float64
		reconnectingPeers float64
	}{
		{
			name: "success",

			expectedBlockGaps: 5.0,
			connectedPeers:    1.0,
			reconnectingPeers: 2.0,
		},
		{
			name:        "success",
			getStatsErr: errors.New("some error"),

			expectedBlockGaps: 0.0,
			connectedPeers:    0.0,
			reconnectingPeers: 0.0,
		},
	}

	for _, tc := range tt {
		testutils.RunParallel(t, false, tc.name, func(t *testing.T) {
			// given
			blocktxStore := &mocks.BlocktxStoreMock{GetStatsFunc: func(_ context.Context, _ int) (*store.Stats, error) {
				return &store.Stats{CurrentNumOfBlockGaps: 5}, tc.getStatsErr
			}}

			pm := &blocktxmocks.PeerManagerMock{
				CountConnectedPeersFunc: func() uint {
					return 1
				},
				GetPeersFunc: func() []p2p.PeerI {
					return []p2p.PeerI{
						&p2p_mocks.PeerIMock{},
						&p2p_mocks.PeerIMock{},
						&p2p_mocks.PeerIMock{},
					}
				},
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
			sut := blocktx.NewStatsCollector(logger, pm, blocktxStore, 5, blocktx.WithStatCollectionInterval(30*time.Millisecond))

			// when
			err := sut.Start()
			time.Sleep(50 * time.Millisecond)

			// then
			require.NoError(t, err)
			require.Equal(t, tc.expectedBlockGaps, testutil.ToFloat64(sut.CurrentNumOfBlockGaps))
			require.Equal(t, tc.connectedPeers, testutil.ToFloat64(sut.ConnectedPeers))
			require.Equal(t, tc.reconnectingPeers, testutil.ToFloat64(sut.ReconnectingPeers))

			// cleanup
			sut.Shutdown()
		})
	}
}
