package blocktx_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx/bcnet"
	"github.com/bitcoin-sv/arc/internal/blocktx/bcnet/blocktx_p2p"
	"github.com/bitcoin-sv/arc/internal/blocktx/mocks"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	storeMocks "github.com/bitcoin-sv/arc/internal/blocktx/store/mocks"
	"github.com/bitcoin-sv/arc/internal/p2p"
	p2pMocks "github.com/bitcoin-sv/arc/internal/p2p/mocks"
	"github.com/bitcoin-sv/arc/internal/testdata"
)

func TestUnorphanRecentWrongOrphans(t *testing.T) {
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
			const unorphanRecentWrongOrphansInterval = 50 * time.Millisecond

			storeMock := &storeMocks.BlocktxStoreMock{
				UnorphanRecentWrongOrphansFunc: func(_ context.Context) ([]*blocktx_api.Block, error) {
					return tc.expectedUnorphanedBlocks, nil
				},
			}

			logger := slog.Default()
			blockProcessCh := make(chan *bcnet.BlockMessagePeer, 10)

			sut, err := blocktx.NewProcessor(logger, storeMock, nil, blockProcessCh, blocktx.WithUnorphanRecentWrongOrphans(true, unorphanRecentWrongOrphansInterval))
			require.NoError(t, err)

			// when
			err = blocktx.UnorphanRecentWrongOrphans(sut)
			require.NoError(t, err)

			// then
			sut.Shutdown()
			actualUnorphanedBlocks, err := storeMock.UnorphanRecentWrongOrphans(context.Background())
			require.NoError(t, err)
			require.Equal(t, len(tc.expectedUnorphanedBlocks), len(actualUnorphanedBlocks))
		})
	}
}

func TestBlockGaps(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name            string
		blockGaps       []*store.BlockGap
		getBlockGapsErr error
		retentionDays   int

		expectedBlockGaps int
		expectedError     error
	}{
		{
			name: "success",
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
			retentionDays: 5,

			expectedBlockGaps: 2,
		},
		{
			name:          "no block gaps",
			blockGaps:     []*store.BlockGap{},
			retentionDays: 5,
		},
		{
			name:          "0 retention days",
			blockGaps:     []*store.BlockGap{},
			retentionDays: 0,
		},
		{
			name:            "error getting block gaps",
			getBlockGapsErr: errors.New("failed to get block gaps"),
			retentionDays:   5,

			expectedError: blocktx.ErrGetBlockGapsFailed,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			storeMock := &storeMocks.BlocktxStoreMock{
				GetBlockGapsFunc: func(_ context.Context, _ int) ([]*store.BlockGap, error) {
					return tc.blockGaps, tc.getBlockGapsErr
				},
			}
			const fillGapsInterval = 50 * time.Millisecond
			logger := slog.Default()
			peerMock := &p2pMocks.PeerIMock{
				StringFunc: func() string {
					return ""
				},
			}
			peers := []p2p.PeerI{peerMock}

			pm := &mocks.PeerManagerMock{
				GetPeersFunc: func() []p2p.PeerI {
					return peers
				},
			}
			blockRequestCh := make(chan blocktx_p2p.BlockRequest, 10)
			//blockProcessCh := make(chan *bcnet.BlockMessagePeer, 10)
			sut, err := blocktx.NewProcessor(
				logger,
				storeMock,
				blockRequestCh,
				nil,
				blocktx.WithRetentionDays(tc.retentionDays),
				blocktx.WithFillGaps(true, pm, fillGapsInterval), blocktx.WithRetentionDays(5),
			)
			require.NoError(t, err)

			// when
			// check block gaps
			checkBlockGapsErr := blocktx.CheckBlockGaps(sut)
			gaps := sut.GetBlockGaps()
			// then
			require.Len(t, gaps, len(tc.blockGaps))
			if tc.expectedError != nil {
				require.ErrorIs(t, checkBlockGapsErr, tc.expectedError)
				return
			}
			require.NoError(t, checkBlockGapsErr)

			// when
			// check block gaps
			fillGapsErr := blocktx.FillGaps(sut)
			if tc.expectedError != nil {
				require.ErrorIs(t, fillGapsErr, tc.expectedError)
				return
			}

			require.NoError(t, fillGapsErr)

			// then
			actualBlockGaps := 0
			for range tc.blockGaps {
				<-blockRequestCh
				actualBlockGaps++
			}
			require.Equal(t, actualBlockGaps, tc.expectedBlockGaps)

			sut.Shutdown()
		})
	}
}
