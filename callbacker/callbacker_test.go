package callbacker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/callbacker/callbacker_api"
	"github.com/bitcoin-sv/arc/callbacker/store"
	"github.com/bitcoin-sv/arc/callbacker/store/badgerhold"
	"github.com/bitcoin-sv/arc/callbacker/store/mock"
	"github.com/bitcoin-sv/arc/callbacker/store/mock_gen"
	"github.com/jarcoal/httpmock"
	"github.com/ordishs/go-utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	tx1          = "b042f298deabcebbf15355aa3a13c7d7cfe96c44ac4f492735f936f8e50d06f6"
	tx1Bytes, _  = utils.DecodeAndReverseHexString(tx1)
	testURL      = "http://localhost:8000"
	testCallback = &callbacker_api.Callback{
		Hash:        tx1Bytes,
		Url:         testURL,
		Token:       "token",
		Status:      5,
		BlockHash:   []byte("blockHash"),
		BlockHeight: 324534,
	}
)

//go:generate moq -pkg mock_gen -out ./store/mock_gen/mock.go ./store Store

func TestNewCallbacker(t *testing.T) {
	tt := []struct {
		name  string
		store store.Store

		expectedErrorStr string
	}{
		{
			name:  "no store",
			store: nil,

			expectedErrorStr: "store is nil",
		},
		{
			name:  "store given",
			store: &mock_gen.StoreMock{},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New(tc.store)
			if tc.expectedErrorStr != "" || err != nil {
				require.ErrorContains(t, err, tc.expectedErrorStr)
				return
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCallbacker_AddCallback(t *testing.T) {
	tt := []struct {
		name      string
		responder httpmock.Responder
		setErr    error

		expectedErrorStr  string
		expectedNrOfPosts int
	}{
		{
			name:      "success",
			responder: httpmock.NewStringResponder(200, "OK"),
			// expectedErrGet: store.ErrNotFound,
			expectedNrOfPosts: 1,
		},
		{
			name:      "http - error",
			responder: httpmock.NewErrorResponder(fmt.Errorf("error")),

			expectedNrOfPosts: 1,
		},
		{
			name:   "set key error",
			setErr: errors.New("failed to set key"),

			expectedErrorStr:  "failed to set key",
			expectedNrOfPosts: 0,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()

			httpmock.RegisterResponder(
				"POST",
				testURL,
				tc.responder,
			)

			mockStore := &mock_gen.StoreMock{
				SetFunc: func(ctx context.Context, callback *callbacker_api.Callback) (string, error) {
					return "ffdK2n44BwsyCrz9jTH12fxuEGoLYhDh", tc.setErr
				},
				UpdateExpiryFunc: func(ctx context.Context, key string) error {
					return nil
				},
				DelFunc: func(ctx context.Context, key string) error {
					return nil
				},
			}

			cb, err := New(mockStore)
			require.NoError(t, err)

			var key string
			key, err = cb.AddCallback(context.Background(), testCallback)

			// wait for the initial callback to be sent
			time.Sleep(40 * time.Millisecond)

			info := httpmock.GetCallCountInfo()
			assert.Equal(t, tc.expectedNrOfPosts, info[fmt.Sprintf("POST %s", testURL)])

			if tc.expectedErrorStr != "" || err != nil {
				require.ErrorContains(t, err, tc.expectedErrorStr)
				return
			} else {
				require.NoError(t, err)
			}

			assert.IsType(t, "", key)
		})
	}
}

func TestCallbacker_Start(t *testing.T) {
	tt := []struct {
		name            string
		responder       httpmock.Responder
		getExpired      error
		updateExpiryErr error

		expectedNrOfPosts int
	}{
		{
			name:      "success",
			responder: httpmock.NewStringResponder(200, "OK"),

			expectedNrOfPosts: 1,
		},
		{
			name:       "get expired fails",
			getExpired: errors.New("failed to get expired"),

			expectedNrOfPosts: 0,
		},
		{
			name:      "callback fails",
			responder: httpmock.NewStringResponder(501, "Not Implemented"),

			expectedNrOfPosts: 1,
		},
		{
			name:            "callback 501 status - update expiry fails",
			responder:       httpmock.NewStringResponder(501, "Not Implemented"),
			updateExpiryErr: errors.New("failed update expiry"),

			expectedNrOfPosts: 1,
		},
		{
			name:      "callback fails - update expiry succeeds",
			responder: httpmock.NewErrorResponder(errors.New("not found")),

			expectedNrOfPosts: 1,
		},
		{
			name:            "callback fails - update expiry fails",
			responder:       httpmock.NewErrorResponder(errors.New("not found")),
			updateExpiryErr: errors.New("failed update expiry"),

			expectedNrOfPosts: 1,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()

			httpmock.RegisterResponder(
				"POST",
				testURL,
				tc.responder,
			)

			store := &mock_gen.StoreMock{
				GetExpiredFunc: func(contextMoqParam context.Context) (map[string]*callbacker_api.Callback, error) {
					storedCallback := map[string]*callbacker_api.Callback{"ffdK2n44BwsyCrz9jTH12fxuEGoLYhDh": testCallback}

					return storedCallback, tc.getExpired
				},
				DelFunc: func(ctx context.Context, key string) error {
					require.Equal(t, "ffdK2n44BwsyCrz9jTH12fxuEGoLYhDh", key)

					return nil
				},
				UpdateExpiryFunc: func(ctx context.Context, key string) error {
					return tc.updateExpiryErr
				},
			}

			cb, err := New(store,
				WithLogger(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))),
				WithSendCallbacksInterval(time.Millisecond*20),
			)
			require.NoError(t, err)

			cb.Start()
			defer cb.Stop()

			time.Sleep(time.Millisecond * 30)

			info := httpmock.GetCallCountInfo()
			assert.Equal(t, tc.expectedNrOfPosts, info[fmt.Sprintf("POST %s", testURL)])
		})
	}
}

func TestCallbacker_sendCallbacksWithBadgerHold(t *testing.T) {
	tt := []struct {
		name        string
		timeToSleep time.Duration

		expectedNrOfCalls int
		expectedErrGet    error
	}{
		{
			name:        "send callbacks - not sent, too new",
			timeToSleep: time.Millisecond * 1,

			expectedNrOfCalls: 0,
			expectedErrGet:    nil,
		},
		{
			name:        "send callbacks - sent",
			timeToSleep: time.Millisecond * 11,

			expectedNrOfCalls: 1,
			expectedErrGet:    store.ErrNotFound,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()

			httpmock.RegisterResponder(
				"POST",
				testURL,
				httpmock.NewStringResponder(200, "OK"),
			)

			dataDir, err := os.MkdirTemp("", "TestCallbacker_sendCallbacks")
			require.NoError(t, err)
			mockStore, err := badgerhold.New(dataDir, 10*time.Millisecond)
			require.NoError(t, err)
			defer func() {
				_ = mockStore.Close(context.Background())
				err = os.RemoveAll(dataDir)
				assert.NoError(t, err)
			}()

			key, err := mockStore.Set(context.Background(), testCallback)
			require.NoError(t, err)

			cb, err := New(mockStore)
			require.NoError(t, err)

			time.Sleep(tc.timeToSleep)

			err = cb.sendCallbacks()
			assert.NoError(t, err)

			info := httpmock.GetCallCountInfo()
			assert.Equal(t, tc.expectedNrOfCalls, info[fmt.Sprintf("POST %s", testURL)])

			var data *callbacker_api.Callback
			data, err = mockStore.Get(context.Background(), key)
			if tc.expectedErrGet != nil {
				assert.ErrorIs(t, err, tc.expectedErrGet)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, testCallback.GetUrl(), data.GetUrl())
			assert.Equal(t, testCallback.GetToken(), data.GetToken())
			assert.Equal(t, testCallback.GetStatus(), data.GetStatus())
		})
	}
}

func TestCallbacker_sendCallback(t *testing.T) {
	tt := []struct {
		name           string
		responder      httpmock.Responder
		expectedErrGet error
	}{
		{
			name:           "success",
			responder:      httpmock.NewStringResponder(200, "OK"),
			expectedErrGet: store.ErrNotFound,
		},
		{
			name:      "error",
			responder: httpmock.NewStringResponder(501, "Not Implemented"),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()

			httpmock.RegisterResponder(
				"POST",
				testURL,
				tc.responder,
			)

			mockStore, err := mock.New()
			require.NoError(t, err)

			key, err := mockStore.Set(context.Background(), testCallback)
			require.NoError(t, err)

			cb, err := New(mockStore)
			require.NoError(t, err)

			err = cb.sendCallback(key, testCallback)
			assert.NoError(t, err)

			info := httpmock.GetCallCountInfo()
			assert.Equal(t, 1, info[fmt.Sprintf("POST %s", testURL)])

			_, err = mockStore.Get(context.Background(), key)
			if tc.expectedErrGet != nil {
				assert.ErrorIs(t, err, tc.expectedErrGet)
				return
			}

			assert.NoError(t, err)
		})
	}
}
