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
			name:          "valid logger - INFO",
			loglevel:      "INFO",
			logformat:     "text",
			expectedError: nil,
		},
		{
			name:          "valid logger - INFO",
			loglevel:      "INFO",
			logformat:     "json",
			expectedError: nil,
		},
		{
			name:          "valid logger - WARN",
			loglevel:      "WARN",
			logformat:     "text",
			expectedError: nil,
		},
		{
			name:          "valid logger - ERROR",
			loglevel:      "ERROR",
			logformat:     "text",
			expectedError: nil,
		},
		{
			name:          "valid logger - DEBUG",
			loglevel:      "DEBUG",
			logformat:     "text",
			expectedError: nil,
		},
		{
			name:          "valid logger - TRACE",
			loglevel:      "TRACE",
			logformat:     "tint",
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
				sut.Info("test" + tc.name)
			}

			// then
			assert.ErrorIs(t, err, tc.expectedError)
			if tc.expectedError == nil {
				if tc.loglevel == "ERROR" {
					assert.Equal(t, sut.Enabled(context.Background(), slog.LevelError), true)
				}
				if tc.loglevel == "INFO" {
					assert.Equal(t, sut.Enabled(context.Background(), slog.LevelInfo), true)
				}
				if tc.loglevel == "WARN" {
					assert.Equal(t, sut.Enabled(context.Background(), slog.LevelWarn), true)
				}
				if tc.loglevel == "DEBUG" {
					assert.Equal(t, sut.Enabled(context.Background(), slog.LevelDebug), true)
				}
				if tc.loglevel == "TRACE" {
					assert.Equal(t, sut.Enabled(context.Background(), slog.LevelInfo), true)
				}

			}
		})
	}
}
