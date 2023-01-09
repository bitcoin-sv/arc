package badgerhold

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/TAAL-GmbH/arc/callbacker/callbacker_api"
	"github.com/TAAL-GmbH/arc/callbacker/store"
	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/labstack/gommon/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testCallback = &callbacker_api.Callback{
	Hash:   []byte("test hash"),
	Url:    "url",
	Token:  "token",
	Status: int32(metamorph_api.Status_SENT_TO_NETWORK),
}

func TestBadgerHold_Get(t *testing.T) {
	t.Run("get - not found", func(t *testing.T) {
		bh, tearDown := setupSuite(t)
		defer tearDown(t)

		_, err := bh.Get(context.Background(), "hello world")
		require.ErrorIs(t, store.ErrNotFound, err)
	})

	t.Run("get", func(t *testing.T) {
		bh, tearDown := setupSuite(t)
		defer tearDown(t)

		key, err := bh.Set(context.Background(), testCallback)

		require.NoError(t, err)

		var data *callbacker_api.Callback
		data, err = bh.Get(context.Background(), key)
		require.NoError(t, err)
		assert.Equal(t, "test hash", string(data.Hash))
		assert.Equal(t, "url", data.Url)
		assert.Equal(t, "token", data.Token)
		assert.Equal(t, int32(metamorph_api.Status_SENT_TO_NETWORK), data.Status)
	})
}

func TestBadgerHold_GetExpired(t *testing.T) {
	t.Run("get - empty", func(t *testing.T) {
		bh, tearDown := setupSuite(t)
		defer tearDown(t)

		expired, err := bh.GetExpired(context.Background())
		require.NoError(t, err)
		assert.Empty(t, expired)
	})

	t.Run("get expired", func(t *testing.T) {
		bh, tearDown := setupSuite(t)
		defer tearDown(t)

		key1, _ := bh.Set(context.Background(), testCallback)
		key2, _ := bh.Set(context.Background(), &callbacker_api.Callback{
			Hash:   []byte("test hash 2"),
			Url:    "url 2",
			Token:  "token 2",
			Status: int32(metamorph_api.Status_SENT_TO_NETWORK),
		})
		time.Sleep(10 * time.Millisecond)

		expired, err := bh.GetExpired(context.Background())
		require.NoError(t, err)
		assert.Len(t, expired, 2)
		assert.Equal(t, "test hash", string(expired[key1].Hash))
		assert.Equal(t, "test hash 2", string(expired[key2].Hash))
	})
}

func TestBadgerHold_Set(t *testing.T) {
	t.Run("set nil", func(t *testing.T) {
		bh, tearDown := setupSuite(t)
		defer tearDown(t)

		_, err := bh.Set(context.Background(), nil)
		require.Error(t, err)
	})

	t.Run("set empty", func(t *testing.T) {
		bh, tearDown := setupSuite(t)
		defer tearDown(t)

		key, err := bh.Set(context.Background(), &callbacker_api.Callback{})
		require.NoError(t, err)
		assert.NotEmpty(t, key)
	})

	t.Run("set", func(t *testing.T) {
		bh, tearDown := setupSuite(t)
		defer tearDown(t)

		key, err := bh.Set(context.Background(), testCallback)
		require.NoError(t, err)

		var data *callbacker_api.Callback
		data, err = bh.Get(context.Background(), key)
		require.NoError(t, err)
		assert.Equal(t, testCallback.Hash, data.Hash)
	})
}

func TestBadgerHold_Del(t *testing.T) {
	t.Run("del nil", func(t *testing.T) {
		bh, tearDown := setupSuite(t)
		defer tearDown(t)

		err := bh.Del(context.Background(), "test")
		require.Error(t, err)
	})

	t.Run("del key", func(t *testing.T) {
		bh, tearDown := setupSuite(t)
		defer tearDown(t)

		key, _ := bh.Set(context.Background(), testCallback)
		data, _ := bh.Get(context.Background(), key)
		assert.Equal(t, testCallback.Hash, data.Hash)

		err := bh.Del(context.Background(), key)
		require.NoError(t, err)

		_, err = bh.Get(context.Background(), key)
		require.ErrorIs(t, store.ErrNotFound, err)
	})
}

func Test_incrementInterval(t *testing.T) {
	t.Run("increment interval", func(t *testing.T) {
		bh, tearDown := setupSuite(t)
		defer tearDown(t)

		duration := time.Second * 1

		interval := bh.incrementInterval(duration, 0)
		assert.Equal(t, time.Second, interval)

		interval = bh.incrementInterval(duration, 1)
		assert.Equal(t, time.Second*2, interval)

		interval = bh.incrementInterval(duration, 2)
		assert.Equal(t, time.Second*4, interval)

		interval = bh.incrementInterval(duration, 3)
		assert.Equal(t, time.Second*8, interval)
	})
}

func setupSuite(t *testing.T) (*BadgerHold, func(t *testing.T)) {
	dataDir := "./data-" + random.String(10)

	err := os.RemoveAll(dataDir)
	require.NoErrorf(t, err, "Could not delete old test data")

	var bh *BadgerHold
	bh, err = New(dataDir, 2)
	require.NoErrorf(t, err, "could not init badgerhold store")

	// Return a function to tear down the test
	return bh, func(t *testing.T) {
		_ = bh.Close(context.Background())
		err = os.RemoveAll(dataDir)
		require.NoErrorf(t, err, "Could not delete old test data")
	}
}

func TestBadgerHold_UpdateExpiry(t *testing.T) {
	t.Run("update expiry - no key", func(t *testing.T) {
		bh, tearDown := setupSuite(t)
		defer tearDown(t)

		err := bh.UpdateExpiry(context.Background(), "key")
		require.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("update expiry", func(t *testing.T) {
		bh, tearDown := setupSuite(t)
		defer tearDown(t)

		key, _ := bh.Set(context.Background(), testCallback)

		err := bh.UpdateExpiry(context.Background(), key)
		require.NoError(t, err)
	})

	t.Run("update expiry - max retries", func(t *testing.T) {
		bh, tearDown := setupSuite(t)
		defer tearDown(t)

		key, _ := bh.Set(context.Background(), testCallback)

		for i := 0; i < bh.maxCallbackRetries; i++ {
			err := bh.UpdateExpiry(context.Background(), key)
			require.NoError(t, err)
		}

		err := bh.UpdateExpiry(context.Background(), key)
		require.ErrorIs(t, err, store.ErrMaxRetries)
	})
}
