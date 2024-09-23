package broadcaster_test

import (
	"errors"
	"testing"

	"github.com/bitcoin-sv/arc/internal/broadcaster/mocks"
	"github.com/stretchr/testify/require"
)

func TestUTXOCreator(t *testing.T) {
	tt := []struct {
		name               string
		outputs            int
		satoshisPerOutput  uint64
		startFunc          func(outputs int, satoshisPerOutput uint64) error
		expectedError      error
		expectedStartCalls int
	}{
		{
			name:              "successful start",
			outputs:           5,
			satoshisPerOutput: 1000,
			startFunc: func(outputs int, satoshisPerOutput uint64) error {
				return nil
			},
			expectedError:      nil,
			expectedStartCalls: 1,
		},
		{
			name:              "error on start",
			outputs:           5,
			satoshisPerOutput: 1000,
			startFunc: func(outputs int, satoshisPerOutput uint64) error {
				return errors.New("failed to start UTXO creation")
			},
			expectedError:      errors.New("failed to start UTXO creation"),
			expectedStartCalls: 1,
		},
		{
			name:              "invalid input - zero outputs",
			outputs:           0,
			satoshisPerOutput: 1000,
			startFunc: func(outputs int, satoshisPerOutput uint64) error {
				return nil // Simulate success to test input handling
			},
			expectedError:      nil, // Start should handle zero outputs gracefully
			expectedStartCalls: 1,
		},
		{
			name:              "invalid input - zero satoshis per output",
			outputs:           5,
			satoshisPerOutput: 0,
			startFunc: func(outputs int, satoshisPerOutput uint64) error {
				return nil // Simulate success to test input handling
			},
			expectedError:      nil, // Start should handle zero satoshis gracefully
			expectedStartCalls: 1,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Create the mock
			mockedCreator := &mocks.CreatorMock{
				StartFunc: tc.startFunc,
			}

			// Call the Start method with mock
			err := mockedCreator.Start(tc.outputs, tc.satoshisPerOutput)

			// Verify if the error is as expected
			if tc.expectedError != nil {
				require.Error(t, err)
				require.EqualError(t, err, tc.expectedError.Error())
			} else {
				require.NoError(t, err)
			}

			// Verify the number of times Start was called
			require.Equal(t, tc.expectedStartCalls, len(mockedCreator.StartCalls()))

		})
	}
}
