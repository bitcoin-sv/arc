package blocktx

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

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
	}{
		{
			name: "success",

			expectedBlockGaps: 5.0,
		},
		{
			name:        "success",
			getStatsErr: errors.New("some error"),

			expectedBlockGaps: 0.0,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			blocktxStore := &mocks.BlocktxStoreMock{GetStatsFunc: func(_ context.Context) (*store.Stats, error) {
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
			sut := NewStatsCollector(logger, pm, blocktxStore, WithStatCollectionInterval(30*time.Millisecond))

			// when
			err := sut.Start()
			time.Sleep(50 * time.Millisecond)

			// then
			require.NoError(t, err)
			require.Equal(t, tc.expectedBlockGaps, testutil.ToFloat64(sut.currentNumOfBlockGaps))
			require.Equal(t, 1.0, testutil.ToFloat64(sut.connectedPeers))
			require.Equal(t, 3.0, testutil.ToFloat64(sut.reconnectingPeers))

			// cleanup
			sut.Shutdown()
		})
	}
}
