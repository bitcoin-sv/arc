package callbacker

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/TAAL-GmbH/arc/callbacker/callbacker_api"
	"github.com/TAAL-GmbH/arc/callbacker/store"
	"github.com/TAAL-GmbH/arc/callbacker/store/badgerhold"
	"github.com/TAAL-GmbH/arc/callbacker/store/mock"
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

func TestNewCallbacker(t *testing.T) {
	t.Run("new callbacker - no store", func(t *testing.T) {
		_, err := NewCallbacker(nil)
		assert.Error(t, err)
	})

	t.Run("new callbacker", func(t *testing.T) {
		store, err := mock.New()
		require.NoError(t, err)

		_, err = NewCallbacker(store)
		assert.NoError(t, err)
	})
}

func TestCallbacker_AddCallback(t *testing.T) {
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

		cb, err := NewCallbacker(mockStore)
		require.NoError(t, err)

		var key string
		key, err = cb.AddCallback(context.Background(), testCallback)
		assert.NoError(t, err)
		assert.IsType(t, "", key)

		// wait for the initial callback to be sent
		time.Sleep(10 * time.Millisecond)

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

		cb, err := NewCallbacker(mockStore)
		require.NoError(t, err)

		var key string
		key, err = cb.AddCallback(context.Background(), testCallback)
		assert.NoError(t, err)
		assert.IsType(t, "", key)

		// wait for the initial callback to be sent
		time.Sleep(10 * time.Millisecond)

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
	t.Run("send callbacks - not sent, too new", func(t *testing.T) {
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

		cb, err := NewCallbacker(mockStore)
		require.NoError(t, err)

		err = cb.sendCallbacks()
		assert.NoError(t, err)

		info := httpmock.GetCallCountInfo()
		assert.Equal(t, 0, info[fmt.Sprintf("POST %s", testURL)])

		var data *callbacker_api.Callback
		data, err = mockStore.Get(context.Background(), key)
		assert.NoError(t, err)
		assert.Equal(t, testCallback.Url, data.Url)
		assert.Equal(t, testCallback.Token, data.Token)
		assert.Equal(t, testCallback.Status, data.Status)
	})

	t.Run("send callbacks - not sent, too new", func(t *testing.T) {
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

		cb, err := NewCallbacker(mockStore)
		require.NoError(t, err)

		time.Sleep(11 * time.Millisecond)

		err = cb.sendCallbacks()
		assert.NoError(t, err)

		info := httpmock.GetCallCountInfo()
		assert.Equal(t, 1, info[fmt.Sprintf("POST %s", testURL)])

		_, err = mockStore.Get(context.Background(), key)
		assert.ErrorIs(t, err, store.ErrNotFound)
	})
}

func TestCallbacker_sendCallback(t *testing.T) {
	t.Run("send callback", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder(
			"POST",
			testURL,
			httpmock.NewStringResponder(200, "OK"),
		)

		mockStore, err := mock.New()
		require.NoError(t, err)

		key, err := mockStore.Set(context.Background(), testCallback)
		require.NoError(t, err)

		cb, err := NewCallbacker(mockStore)
		require.NoError(t, err)

		err = cb.sendCallback(key, testCallback)
		assert.NoError(t, err)

		info := httpmock.GetCallCountInfo()
		assert.Equal(t, 1, info[fmt.Sprintf("POST %s", testURL)])

		_, err = mockStore.Get(context.Background(), key)
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("send callback - error", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder(
			"POST",
			testURL,
			httpmock.NewStringResponder(501, "Not Implemented"),
		)

		mockStore, err := mock.New()
		require.NoError(t, err)

		key, err := mockStore.Set(context.Background(), testCallback)
		require.NoError(t, err)

		cb, err := NewCallbacker(mockStore)
		require.NoError(t, err)

		err = cb.sendCallback(key, testCallback)
		assert.NoError(t, err)

		info := httpmock.GetCallCountInfo()
		assert.Equal(t, 1, info[fmt.Sprintf("POST %s", testURL)])

		_, err = mockStore.Get(context.Background(), key)
		assert.NoError(t, err)
	})
}
