package blocktx_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/blocktx/mocks"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	storeMocks "github.com/bitcoin-sv/arc/internal/blocktx/store/mocks"
	"github.com/bitcoin-sv/arc/internal/testdata"
	"github.com/libsv/go-p2p"
	"github.com/stretchr/testify/require"
)

func TestStartFillGaps(t *testing.T) {
	hostname, err := os.Hostname()
	require.NoError(t, err)

	tt := []struct {
		name            string
		hostname        string
		getBlockGapsErr error
		blockGaps       []*store.BlockGap

		expectedGetBlockGapsCalls int
	}{
		{
			name:     "success",
			hostname: hostname,
			blockGaps: []*store.BlockGap{
				{
					Height: 822014,
					Hash:   testdata.Block1Hash,
				},
				{
					Height: 822015,
					Hash:   testdata.Block2Hash,
				},
			},

			expectedGetBlockGapsCalls: 1,
		},
		{
			name:            "error getting block gaps",
			hostname:        hostname,
			getBlockGapsErr: errors.New("failed to get block gaps"),

			expectedGetBlockGapsCalls: 1,
		},
		{
			name:      "no block gaps",
			hostname:  hostname,
			blockGaps: make([]*store.BlockGap, 0),

			expectedGetBlockGapsCalls: 3,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			getBlockErrCh := make(chan error)

			getBlockGapTestErr := tc.getBlockGapsErr
			storeMock := &storeMocks.BlocktxStoreMock{
				GetBlockGapsFunc: func(_ context.Context, _ int) ([]*store.BlockGap, error) {
					if getBlockGapTestErr != nil {
						getBlockErrCh <- getBlockGapTestErr
						return nil, getBlockGapTestErr
					}

					return tc.blockGaps, nil
				},
			}

			peerMock := &mocks.PeerMock{
				StringFunc: func() string {
					return ""
				},
			}
			peers := []p2p.PeerI{peerMock}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
			blockRequestCh := make(chan blocktx.BlockRequest, 10)

			sut := blocktx.NewBackgroundWorkers(storeMock, logger)

			interval := 50 * time.Millisecond

			// when
			sut.StartFillGaps(peers, interval, 28, blockRequestCh)

			// then
			select {
			case hashPeer := <-blockRequestCh:
				require.True(t, testdata.Block1Hash.IsEqual(hashPeer.Hash))
			case err = <-getBlockErrCh:
				require.ErrorIs(t, err, tc.getBlockGapsErr)
			case <-time.After(time.Duration(3.5 * float64(interval))):
			}

			sut.GracefulStop()
			require.Equal(t, tc.expectedGetBlockGapsCalls, len(storeMock.GetBlockGapsCalls()))
		})
	}
}