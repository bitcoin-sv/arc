package metamorph_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/bitcoin-sv/arc/internal/metamorph/mocks"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
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
			mtmStore := &mocks.MetamorphStoreMock{
				GetStatsFunc: func(ctx context.Context, since time.Time) (*store.Stats, error) {
					return &store.Stats{
						StatusStored:              15,
						StatusAnnouncedToNetwork:  20,
						StatusRequestedByNetwork:  100,
						StatusSentToNetwork:       30,
						StatusAcceptedByNetwork:   21,
						StatusSeenOnNetwork:       55,
						StatusMined:               75,
						StatusConfirmed:           123,
						StatusRejected:            683,
						StatusSeenInOrphanMempool: 8,
					}, tc.getStatsErr
				},
			}

			pm := &mocks.PeerManagerMock{}
			processor, err := metamorph.NewProcessor(mtmStore, pm,
				metamorph.WithProcessorLogger(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: metamorph.LogLevelDefault}))),
				metamorph.WithStatCollectionInterval(10*time.Millisecond),
			)
			require.NoError(t, err)

			err = processor.StartCollectStats()
			require.NoError(t, err)

			time.Sleep(25 * time.Millisecond)

			processor.Shutdown()
		})
	}
}
