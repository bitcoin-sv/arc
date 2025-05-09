package blocktx_test

import (
	"context"
	"errors"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/blocktx/bcnet/blocktx_p2p"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	storeMocks "github.com/bitcoin-sv/arc/internal/blocktx/store/mocks"
	"github.com/bitcoin-sv/arc/internal/p2p"
	p2pMocks "github.com/bitcoin-sv/arc/internal/p2p/mocks"
	"github.com/bitcoin-sv/arc/internal/testdata"
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
			const fillGapsInterval = 50 * time.Millisecond

			blockRequestingCh := make(chan blocktx_p2p.BlockRequest, 10)
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

			peerMock := &p2pMocks.PeerIMock{
				StringFunc: func() string {
					return ""
				},
			}
			peers := []p2p.PeerI{peerMock}

			sut := blocktx.NewBackgroundWorkers(storeMock, slog.Default())

			// when
			sut.StartFillGaps(peers, fillGapsInterval, 28, blockRequestingCh)

			// then
			select {
			case hashPeer := <-blockRequestingCh:
				require.True(t, testdata.Block1Hash.IsEqual(hashPeer.Hash))
			case err = <-getBlockErrCh:
				require.ErrorIs(t, err, tc.getBlockGapsErr)
			case <-time.After(time.Duration(3.5 * float64(fillGapsInterval))):
			}

			sut.GracefulStop()
			require.Equal(t, tc.expectedGetBlockGapsCalls, len(storeMock.GetBlockGapsCalls()))
		})
	}
}

func TestStartUnorphanRecentWrongOrphans(t *testing.T) {
	tt := []struct {
		name                     string
		expectedUnorphanedBlocks []*blocktx_api.Block
	}{
		{
			name: "success",
			expectedUnorphanedBlocks: []*blocktx_api.Block{
				{
					Height: 822014,
				},
				{
					Height: 822015,
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			const fillUnorphanRecentWrongOrphansInterval = 50 * time.Millisecond

			storeMock := &storeMocks.BlocktxStoreMock{
				UnorphanRecentWrongOrphansFunc: func(_ context.Context) ([]*blocktx_api.Block, error) {
					return tc.expectedUnorphanedBlocks, nil
				},
			}

			sut := blocktx.NewBackgroundWorkers(storeMock, slog.Default())

			// when
			sut.StartUnorphanRecentWrongOrphans(fillUnorphanRecentWrongOrphansInterval)

			// then
			sut.GracefulStop()
			actualUnorphanedBlocks, err := storeMock.UnorphanRecentWrongOrphans(context.Background())
			require.NoError(t, err)
			require.Equal(t, len(tc.expectedUnorphanedBlocks), len(actualUnorphanedBlocks))
		})
	}
}
