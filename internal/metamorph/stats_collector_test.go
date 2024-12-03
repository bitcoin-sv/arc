package metamorph_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/p2p"

	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	storeMocks "github.com/bitcoin-sv/arc/internal/metamorph/store/mocks"
	"github.com/stretchr/testify/require"
)

func TestStartCollectStats(t *testing.T) {
	tt := []struct {
		name        string
		getStatsErr error
	}{
		{
			name: "success",
		},
		{
			name:        "error - failed to get stats",
			getStatsErr: errors.New("some error"),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			mtmStore := &storeMocks.MetamorphStoreMock{
				GetStatsFunc: func(_ context.Context, _ time.Time, _ time.Duration, _ time.Duration) (*store.Stats, error) {
					return &store.Stats{
						StatusStored:              15,
						StatusAnnouncedToNetwork:  20,
						StatusRequestedByNetwork:  100,
						StatusSentToNetwork:       30,
						StatusAcceptedByNetwork:   21,
						StatusSeenOnNetwork:       55,
						StatusMined:               75,
						StatusRejected:            683,
						StatusSeenInOrphanMempool: 8,
					}, tc.getStatsErr
				},
				SetUnlockedByNameFunc: func(_ context.Context, _ string) (int64, error) { return 0, nil },
			}

			messanger := &p2p.NetworkMessenger{}

			processor, err := metamorph.NewProcessor(mtmStore, nil, messanger, nil,
				metamorph.WithProcessorLogger(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: metamorph.LogLevelDefault}))),
				metamorph.WithStatCollectionInterval(10*time.Millisecond),
			)
			require.NoError(t, err)

			// when
			err = processor.StartCollectStats()

			// then
			require.NoError(t, err)
			time.Sleep(25 * time.Millisecond)
			processor.Shutdown()
		})
	}
}
