package callbacker_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/callbacker"
	"github.com/bitcoin-sv/arc/internal/callbacker/mocks"
	"github.com/bitcoin-sv/arc/internal/callbacker/store"
)

func TestStartCallbackStoreCleanup(t *testing.T) {
	tt := []struct {
		name                     string
		deleteFailedOlderThanErr error

		expectedIterations int
	}{
		{
			name: "success",

			expectedIterations: 4,
		},
		{
			name:                     "error deleting failed older than",
			deleteFailedOlderThanErr: errors.New("some error"),

			expectedIterations: 4,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			cbStore := &mocks.CallbackerStoreMock{
				DeleteFailedOlderThanFunc: func(_ context.Context, _ time.Time) error {
					return tc.deleteFailedOlderThanErr
				},
			}
			logger := slog.Default()

			bw := callbacker.NewBackgroundWorkers(cbStore, nil, logger)
			defer bw.GracefulStop()

			bw.StartCallbackStoreCleanup(20*time.Millisecond, 50*time.Second)

			time.Sleep(90 * time.Millisecond)

			require.Equal(t, tc.expectedIterations, len(cbStore.DeleteFailedOlderThanCalls()))
		})
	}
}

func TestStartFailedCallbacksDispatch(t *testing.T) {
	tt := []struct {
		name             string
		storedCallbacks  []*store.CallbackData
		popFailedManyErr error

		expectedIterations int
		expectedSends      int
	}{
		{
			name: "success",
			storedCallbacks: []*store.CallbackData{{
				URL:   "https://test.com",
				Token: "1234",
			}},

			expectedIterations: 4,
			expectedSends:      4,
		},
		{
			name:            "success - no callbacks in store",
			storedCallbacks: []*store.CallbackData{},

			expectedIterations: 4,
			expectedSends:      0,
		},
		{
			name:             "error getting stored callbacks",
			storedCallbacks:  []*store.CallbackData{},
			popFailedManyErr: errors.New("some error"),

			expectedIterations: 4,
			expectedSends:      0,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			cbStore := &mocks.CallbackerStoreMock{
				PopFailedManyFunc: func(_ context.Context, _ time.Time, limit int) ([]*store.CallbackData, error) {
					require.Equal(t, 100, limit)
					return tc.storedCallbacks, tc.popFailedManyErr
				},
				SetFunc: func(_ context.Context, _ *store.CallbackData) error {
					return nil
				},
			}
			sender := &mocks.SenderIMock{
				SendFunc: func(_ string, _ string, _ *callbacker.Callback) (bool, bool) {
					return true, false
				},
			}
			logger := slog.Default()
			dispatcher := callbacker.NewCallbackDispatcher(sender, cbStore, logger, &callbacker.SendConfig{})

			bw := callbacker.NewBackgroundWorkers(cbStore, dispatcher, logger)
			defer bw.GracefulStop()

			bw.StartFailedCallbacksDispatch(20 * time.Millisecond)

			time.Sleep(90 * time.Millisecond)

			require.Equal(t, tc.expectedIterations, len(cbStore.PopFailedManyCalls()))
			require.Equal(t, tc.expectedSends, len(sender.SendCalls()))
		})
	}
}

func TestDispatchPersistedCallbacks(t *testing.T) {
	tt := []struct {
		name            string
		storedCallbacks []*store.CallbackData
		popManyErr      error

		expectedErr        error
		expectedIterations int
		expectedSends      int
	}{
		{
			name:            "success - no stored callbacks",
			storedCallbacks: []*store.CallbackData{},

			expectedIterations: 1,
		},
		{
			name: "success - 1 stored callback",
			storedCallbacks: []*store.CallbackData{{
				URL:   "https://test.com",
				Token: "1234",
			}},

			expectedIterations: 2,
			expectedSends:      1,
		},
		{
			name:       "error deleting failed older than",
			popManyErr: errors.New("some error"),

			expectedErr:        callbacker.ErrFailedPopMany,
			expectedIterations: 1,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			counter := 0

			cbStore := &mocks.CallbackerStoreMock{
				PopManyFunc: func(_ context.Context, limit int) ([]*store.CallbackData, error) {
					require.Equal(t, 100, limit)
					if counter > 0 {
						return []*store.CallbackData{}, nil
					}
					counter++

					return tc.storedCallbacks, tc.popManyErr
				},
				SetFunc: func(_ context.Context, _ *store.CallbackData) error {
					return nil
				},
			}
			sender := &mocks.SenderIMock{
				SendFunc: func(_ string, _ string, _ *callbacker.Callback) (bool, bool) {
					return true, false
				},
			}
			logger := slog.Default()
			dispatcher := callbacker.NewCallbackDispatcher(sender, cbStore, logger, &callbacker.SendConfig{})

			bw := callbacker.NewBackgroundWorkers(cbStore, dispatcher, logger)
			defer bw.GracefulStop()

			actualError := bw.DispatchPersistedCallbacks()

			require.Equal(t, tc.expectedIterations, len(cbStore.PopManyCalls()))
			require.Equal(t, tc.expectedSends, len(sender.SendCalls()))
			if tc.expectedErr == nil {
				require.NoError(t, actualError)
				return
			}
			require.ErrorIs(t, actualError, tc.expectedErr)
		})
	}
}
