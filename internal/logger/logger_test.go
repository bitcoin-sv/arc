package logger

import (
	"context"
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
			name:          "valid logger",
			loglevel:      "INFO",
			logformat:     "json",
			expectedError: nil,
		},
		{
			name:          "invalid log format",
			loglevel:      "INFO",
			logformat:     "invalid format",
			expectedError: ErrLoggerInvalidLogFormat,
		},
		{
			name:          "invalid log level",
			loglevel:      "INVALID_LEVEL",
			logformat:     "text",
			expectedError: ErrLoggerInvalidLogLevel,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			sut, err := NewLogger(tc.loglevel, tc.logformat)

			if sut != nil {
				sut.Info("test")
			}

			// then
			assert.ErrorIs(t, err, tc.expectedError)
			if tc.expectedError == nil {
				assert.Equal(t, sut.Enabled(context.Background(), slog.LevelInfo), true)
			}
		})
	}
}
