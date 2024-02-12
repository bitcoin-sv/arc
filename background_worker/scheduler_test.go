package background_worker

import (
	"errors"
	"github.com/stretchr/testify/require"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/go-co-op/gocron"
)

func TestScheduler_RunJob(t *testing.T) {
	tt := []struct {
		name   string
		jobErr error

		expectedRunJobCount int
	}{
		{
			name: "success",

			expectedRunJobCount: 1,
		},
		{
			name:   "job error",
			jobErr: errors.New("failed to execute job"),

			expectedRunJobCount: 1,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
			scheduler := NewScheduler(gocron.NewScheduler(time.UTC), time.Millisecond*20, logger)

			wg := &sync.WaitGroup{}
			wg.Add(1)
			runJobCount := 0
			scheduler.RunJob("test job", func() error {
				runJobCount++
				wg.Done()
				return tc.jobErr
			})

			scheduler.Start()

			wg.Wait()
			scheduler.Shutdown()

			require.Equal(t, tc.expectedRunJobCount, runJobCount)
		})
	}
}
