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

	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/bitcoin-sv/arc/internal/blocktx/store/mocks"
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
			blocktxStore := &mocks.BlocktxStoreMock{GetStatsFunc: func(ctx context.Context) (*store.Stats, error) {
				return &store.Stats{CurrentNumOfBlockGaps: 5}, tc.getStatsErr
			}}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
			sut := NewStatsCollector(logger, blocktxStore, WithStatCollectionInterval(30*time.Millisecond))

			// when
			err := sut.Start()
			time.Sleep(50 * time.Millisecond)

			// then
			require.NoError(t, err)
			require.Equal(t, tc.expectedBlockGaps, testutil.ToFloat64(sut.currentNumOfBlockGaps))

			// cleanup
			sut.Shutdown()
		})
	}
}
