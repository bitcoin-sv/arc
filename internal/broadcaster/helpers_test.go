package broadcaster

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRampUpTicker(t *testing.T) {
	tt := []struct {
		name          string
		startInterval time.Duration
		endInterval   time.Duration
		steps         int64

		expectedError error
	}{
		{
			name:          "success",
			startInterval: 1 * time.Second,
			endInterval:   100 * time.Millisecond,
			steps:         4,
		},
		{
			name:          "error - end interval greater than start interval",
			startInterval: 3 * time.Second,
			endInterval:   6 * time.Second,
			steps:         6,

			expectedError: ErrStartIntervalNotGreaterEndInterval,
		},
		{
			name:          "error - steps 0",
			startInterval: 3 * time.Second,
			endInterval:   6 * time.Second,
			steps:         0,

			expectedError: ErrStepsZero,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ticker, actualErr := NewRampUpTicker(tc.startInterval, tc.endInterval, tc.steps)
			if tc.expectedError != nil {
				require.ErrorIs(t, actualErr, tc.expectedError)
				return
			}
			require.NoError(t, actualErr)

			tickerCh := ticker.GetTickerCh()

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

			timeSlice := [6]time.Time{}
			counter := 0
		outerLoop:
			for timeStamp := range tickerCh {
				logger.Info("ticker ticked")

				timeSlice[counter] = timeStamp

				counter++

				if counter >= 6 {
					break outerLoop
				}
			}

			const delta = 20 * time.Millisecond

			assert.True(t, timeSlice[1].Sub(timeSlice[0]).Abs() < 775*time.Millisecond+delta && timeSlice[1].Sub(timeSlice[0]).Abs() > 775*time.Millisecond-delta)
			assert.True(t, timeSlice[2].Sub(timeSlice[1]).Abs() < 550*time.Millisecond+delta && timeSlice[2].Sub(timeSlice[1]).Abs() > 550*time.Millisecond-delta)
			assert.True(t, timeSlice[3].Sub(timeSlice[2]).Abs() < 325*time.Millisecond+delta && timeSlice[3].Sub(timeSlice[2]).Abs() > 325*time.Millisecond-delta)
			assert.True(t, timeSlice[4].Sub(timeSlice[3]).Abs() < 100*time.Millisecond+delta && timeSlice[4].Sub(timeSlice[3]).Abs() > 100*time.Millisecond-delta)
			assert.True(t, timeSlice[5].Sub(timeSlice[4]).Abs() < 100*time.Millisecond+delta && timeSlice[5].Sub(timeSlice[4]).Abs() > 100*time.Millisecond-delta)
		})
	}
}
