package callbacker

import (
	"context"
	"fmt"
	"testing"

	"github.com/TAAL-GmbH/arc/callbacker/callbacker_api"
	"github.com/TAAL-GmbH/arc/callbacker/store"
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

func TestCallbacker_sendCallbacks(t *testing.T) {
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
