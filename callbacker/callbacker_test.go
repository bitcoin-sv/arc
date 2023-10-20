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
	"github.com/bitcoin-sv/arc/callbacker/store/mock2"
	"github.com/jarcoal/httpmock"
	"github.com/labstack/gommon/random"
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

//go:generate moq -pkg mock2 -out ./store/mock2/mock.go ./store Store

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
			store: &mock2.StoreMock{},
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
	t.Parallel()

	t.Run("add callback", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder(
			"POST",
			testURL,
			httpmock.NewStringResponder(200, "OK"),
		)

		mockStore, err := mock.New()
		require.NoError(t, err)

		cb, err := New(mockStore)
		require.NoError(t, err)

		var key string
		key, err = cb.AddCallback(context.Background(), testCallback)
		assert.NoError(t, err)
		assert.IsType(t, "", key)

		// wait for the initial callback to be sent
		time.Sleep(40 * time.Millisecond)

		info := httpmock.GetCallCountInfo()
		assert.Equal(t, 1, info[fmt.Sprintf("POST %s", testURL)])

		_, err = mockStore.Get(context.Background(), key)
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("add callback - error and store", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder(
			"POST",
			testURL,
			httpmock.NewErrorResponder(fmt.Errorf("error")),
		)

		mockStore, err := mock.New()
		require.NoError(t, err)

		cb, err := New(mockStore)
		require.NoError(t, err)

		var key string
		key, err = cb.AddCallback(context.Background(), testCallback)
		assert.NoError(t, err)
		assert.IsType(t, "", key)

		// wait for the initial callback to be sent
		time.Sleep(40 * time.Millisecond)

		info := httpmock.GetCallCountInfo()
		assert.Equal(t, 1, info[fmt.Sprintf("POST %s", testURL)])

		// data should still be here, we got an error on the initial callback
		var data *callbacker_api.Callback
		data, err = mockStore.Get(context.Background(), key)
		assert.NoError(t, err)
		assert.Equal(t, testCallback, data)
	})
}

func TestCallbacker_sendCallbacks(t *testing.T) {
	tt := []struct {
		name            string
		getExpired      error
		updateExpiryErr error
		responder       httpmock.Responder

		expectedErrorStr string
	}{
		{
			name:      "not sent, too new",
			responder: httpmock.NewStringResponder(200, "OK"),
		},
		{
			name:       "get expired fails",
			getExpired: errors.New("failed to get expired"),

			expectedErrorStr: "failed to get expired",
		},
		{
			name:      "callback fails",
			responder: httpmock.NewStringResponder(501, "Not Implemented"),
		},
		{
			name:            "callback fails - update expiry fails",
			responder:       httpmock.NewStringResponder(501, "Not Implemented"),
			updateExpiryErr: errors.New("failed update expiry"),
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

			fmt.Println(random.String(32))
			store := &mock2.StoreMock{
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

			cb, err := New(store, WithLogger(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))))
			require.NoError(t, err)

			err = cb.sendCallbacks()
			if tc.expectedErrorStr != "" || err != nil {
				require.ErrorContains(t, err, tc.expectedErrorStr)
				return
			} else {
				require.NoError(t, err)
			}

			info := httpmock.GetCallCountInfo()
			assert.Equal(t, 1, info[fmt.Sprintf("POST %s", testURL)])

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
			assert.Equal(t, testCallback.Url, data.Url)
			assert.Equal(t, testCallback.Token, data.Token)
			assert.Equal(t, testCallback.Status, data.Status)
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

			fmt.Println(key)
			_, err = mockStore.Get(context.Background(), key)
			if tc.expectedErrGet != nil {
				assert.ErrorIs(t, err, tc.expectedErrGet)
				return
			}

			assert.NoError(t, err)
		})
	}
}
