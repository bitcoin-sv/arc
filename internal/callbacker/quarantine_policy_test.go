package callbacker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestQuarantinePolicy(t *testing.T) {
	tt := []struct {
		name           string
		nowFn          func() time.Time
		refTime        time.Time
		expectedResult time.Time
	}{
		{
			name:           "reference point in time close to call time- return base duration",
			nowFn:          func() time.Time { return time.Date(2024, 9, 11, 12, 30, 0, 0, time.UTC) },
			refTime:        time.Date(2024, 9, 11, 12, 29, 0, 0, time.UTC),
			expectedResult: time.Date(2024, 9, 11, 12, 40, 0, 0, time.UTC),
		},
		{
			name:           "reference point in time is earlier than permanent quarantine- return infinity",
			nowFn:          func() time.Time { return time.Date(2024, 9, 11, 12, 30, 0, 0, time.UTC) },
			refTime:        time.Date(2024, 9, 11, 12, 0, 0, 0, time.UTC),
			expectedResult: infinity,
		},
		{
			name:           "reference point in time is eariel than base duration- return double time span",
			nowFn:          func() time.Time { return time.Date(2024, 9, 11, 12, 30, 0, 0, time.UTC) },
			refTime:        time.Date(2024, 9, 11, 12, 15, 0, 0, time.UTC),
			expectedResult: time.Date(2024, 9, 11, 13, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			sut := &quarantinePolicy{
				baseDuration:        10 * time.Minute,
				permQuarantineAfter: 20 * time.Minute,
				now:                 tc.nowFn,
			}

			// when
			actualTime := sut.Until(tc.refTime)

			// then
			require.Equal(t, tc.expectedResult, actualTime)
		})
	}
}
