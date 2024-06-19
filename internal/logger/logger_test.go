package logger

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NewLogger(t *testing.T) {
	testCases := []struct {
		name          string
		loglevel      string
		logformat     string
		expectedError error
	}{
		{
			name:          "valid logger",
			loglevel:      "INFO",
			logformat:     "text",
			expectedError: nil,
		},
		{
			name:          "invalid log format",
			loglevel:      "INFO",
			logformat:     "invalid format",
			expectedError: errors.New("invalid log format: invalid format"),
		},
		{
			name:          "invalid log level",
			loglevel:      "INVALID_LEVEL",
			logformat:     "text",
			expectedError: errors.New("invalid log level: INVALID_LEVEL"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger, err := NewLogger(tc.loglevel, tc.logformat)

			assert.Equal(t, tc.expectedError, err)

			if tc.expectedError == nil {
				assert.Equal(t, logger.Enabled(context.Background(), slog.LevelInfo), true)
			}
		})
	}
}
