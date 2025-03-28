package broadcaster

import (
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewDynamicTicker(t *testing.T) {
	tt := []struct {
		name          string
		startInterval time.Duration
		endInterval   time.Duration
		steps         int64

		expectedError error
	}{
		{
			name:          "success",
			startInterval: 3 * time.Second,
			endInterval:   1 * time.Second,
			steps:         6,
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

			ticker, actualErr := NewDynamicTicker(tc.startInterval, tc.endInterval, tc.steps)
			tickerCh := ticker.GetTickerCh()
			if tc.expectedError != nil {
				require.ErrorIs(t, actualErr, tc.expectedError)
				return
			}
			require.NoError(t, actualErr)

			counter := 0

		outerLoop:
			for {
				select {
				case <-tickerCh:
					slog.Default().Info("ticker ticked")
					counter++

					if counter >= 10 {
						break outerLoop
					}
				}
			}

		})
	}
}
