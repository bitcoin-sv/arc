package badgerhold

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/labstack/gommon/random"
	"github.com/ordishs/go-utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	tx1         = "b042f298deabcebbf15355aa3a13c7d7cfe96c44ac4f492735f936f8e50d06f6"
	tx1Bytes, _ = utils.DecodeAndReverseHexString(tx1)
)

func TestGet(t *testing.T) {
	t.Run("get - error", func(t *testing.T) {
		bh, tearDown := setupSuite(t)
		defer tearDown(t)

		_, err := bh.Get(context.Background(), []byte("hello world"))
		require.ErrorIs(t, store.ErrNotFound, err)
	})
}

func TestPutGetDelete(t *testing.T) {
	bh, tearDown := setupSuite(t)
	defer tearDown(t)

	data := []byte("Hello world")

	hash := utils.Sha256d(data)

	err := bh.Set(context.Background(), hash, &store.StoreData{
		Hash:  hash,
		RawTx: data,
	})
	require.NoError(t, err)

	data2, err := bh.Get(context.Background(), hash)
	require.NoError(t, err)
	assert.Equal(t, hash, data2.Hash)
	assert.Equal(t, data, data2.RawTx)

	err = bh.Del(context.Background(), hash)
	require.NoError(t, err)
}

func TestPutGetMulti(t *testing.T) {
	bh, tearDown := setupSuite(t)
	defer tearDown(t)

	var wg sync.WaitGroup

	for workerId := 0; workerId < 100; workerId++ {
		wg.Add(1)

		go func(workerId int) {
			defer wg.Done()

			for i := 0; i < 100; i++ {
				data := []byte(fmt.Sprintf("Hello world %d-%d", workerId, i))

				hash := utils.Sha256d(data)

				err := bh.Set(context.Background(), hash, &store.StoreData{
					Hash:  hash,
					RawTx: data,
				})
				require.NoError(t, err)

				// data2, err := bh.Get(context.Background(), hash)
				// require.NoError(t, err)
				// assert.Equal(t, data, data2)

				// err = bh.Del(context.Background(), hash)
				// require.NoError(t, err)
			}
		}(workerId)
	}

	wg.Wait()

}

func TestGetUnseen(t *testing.T) {
	t.Run("no unseen", func(t *testing.T) {
		bh, tearDown := setupSuite(t)
		defer tearDown(t)

		hashes := [][]byte{
			utils.Sha256d([]byte("hello world")),
			utils.Sha256d([]byte("hello again")),
			utils.Sha256d([]byte("hello again again")),
		}

		for _, hash := range hashes {
			err := bh.Set(context.Background(), hash, &store.StoreData{
				Hash:   hash,
				Status: metamorph_api.Status_SEEN_ON_NETWORK,
			})
			require.NoError(t, err)
		}

		unseen := make([]*store.StoreData, 0)
		err := bh.GetUnseen(context.Background(), func(s *store.StoreData) {
			unseen = append(unseen, s)
		})
		require.NoError(t, err)
		assert.Equal(t, 0, len(unseen))

		for _, hash := range hashes {
			err = bh.Del(context.Background(), hash)
			require.NoError(t, err)
		}
	})

	t.Run("multiple unseen", func(t *testing.T) {
		bh, tearDown := setupSuite(t)
		defer tearDown(t)

		hashes := [][]byte{
			utils.Sha256d([]byte("hello world")),
			utils.Sha256d([]byte("hello again")),
			utils.Sha256d([]byte("hello again again")),
		}

		for _, hash := range hashes {
			err := bh.Set(context.Background(), hash, &store.StoreData{
				Hash:   hash,
				Status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
			})
			require.NoError(t, err)
		}

		unseen := make([]*store.StoreData, 0)
		err := bh.GetUnseen(context.Background(), func(s *store.StoreData) {
			unseen = append(unseen, s)
		})
		require.NoError(t, err)
		assert.Equal(t, 3, len(unseen))

		for _, hash := range hashes {
			err = bh.Del(context.Background(), hash)
			require.NoError(t, err)
		}
	})
}
func TestUpdateMined(t *testing.T) {
	t.Run("update mined - not found", func(t *testing.T) {
		bh, tearDown := setupSuite(t)
		defer tearDown(t)

		err := bh.UpdateMined(context.Background(), tx1Bytes, []byte("block hash"), 123)
		require.NoError(t, err) // an error is not thrown if not found
	})

	t.Run("update announced to mined", func(t *testing.T) {
		bh, tearDown := setupSuite(t)
		defer tearDown(t)

		err := bh.Set(context.Background(), tx1Bytes, &store.StoreData{
			Hash:   tx1Bytes,
			Status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
		})
		require.NoError(t, err)

		var data *store.StoreData
		data, err = bh.Get(context.Background(), tx1Bytes)
		require.NoError(t, err)

		assert.Equal(t, metamorph_api.Status_ANNOUNCED_TO_NETWORK, data.Status)
		assert.Equal(t, []byte(nil), data.BlockHash)
		assert.Equal(t, int32(0), data.BlockHeight)

		err = bh.UpdateMined(context.Background(), tx1Bytes, []byte("block hash"), 123)
		require.NoError(t, err)

		data, err = bh.Get(context.Background(), tx1Bytes)
		require.NoError(t, err)
		assert.Equal(t, metamorph_api.Status_MINED, data.Status)
		assert.Equal(t, []byte("block hash"), data.BlockHash)
		assert.Equal(t, int32(123), data.BlockHeight)
	})
}

func TestUpdateStatus(t *testing.T) {
	t.Run("update status - not found", func(t *testing.T) {
		bh, tearDown := setupSuite(t)
		defer tearDown(t)

		err := bh.UpdateStatus(context.Background(), tx1Bytes, metamorph_api.Status_SENT_TO_NETWORK, "")
		require.NoError(t, err) // an error is not thrown if not found
	})

	t.Run("update status", func(t *testing.T) {
		bh, tearDown := setupSuite(t)
		defer tearDown(t)

		err := bh.Set(context.Background(), tx1Bytes, &store.StoreData{
			Hash:   tx1Bytes,
			Status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
		})
		require.NoError(t, err)

		var data *store.StoreData
		data, err = bh.Get(context.Background(), tx1Bytes)
		require.NoError(t, err)

		assert.Equal(t, metamorph_api.Status_ANNOUNCED_TO_NETWORK, data.Status)
		assert.Equal(t, "", data.RejectReason)
		assert.Equal(t, []byte(nil), data.BlockHash)
		assert.Equal(t, int32(0), data.BlockHeight)

		err = bh.UpdateStatus(context.Background(), tx1Bytes, metamorph_api.Status_SENT_TO_NETWORK, "")
		require.NoError(t, err)

		data, err = bh.Get(context.Background(), tx1Bytes)
		require.NoError(t, err)
		assert.Equal(t, metamorph_api.Status_SENT_TO_NETWORK, data.Status)
		assert.Equal(t, "", data.RejectReason)
		assert.Equal(t, []byte(nil), data.BlockHash)
		assert.Equal(t, int32(0), data.BlockHeight)
	})

	t.Run("update status with error", func(t *testing.T) {
		bh, tearDown := setupSuite(t)
		defer tearDown(t)

		err := bh.Set(context.Background(), tx1Bytes, &store.StoreData{
			Hash:   tx1Bytes,
			Status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
		})
		require.NoError(t, err)

		var data *store.StoreData
		data, err = bh.Get(context.Background(), tx1Bytes)
		require.NoError(t, err)

		assert.Equal(t, metamorph_api.Status_ANNOUNCED_TO_NETWORK, data.Status)
		assert.Equal(t, "", data.RejectReason)

		err = bh.UpdateStatus(context.Background(), tx1Bytes, metamorph_api.Status_REJECTED, "error encountered")
		require.NoError(t, err)

		data, err = bh.Get(context.Background(), tx1Bytes)
		require.NoError(t, err)
		assert.Equal(t, metamorph_api.Status_REJECTED, data.Status)
		assert.Equal(t, "error encountered", data.RejectReason)
	})
}

func setupSuite(t *testing.T) (store.Store, func(t *testing.T)) {
	dataDir := "./data-" + random.String(10)

	err := os.RemoveAll(dataDir)
	require.NoErrorf(t, err, "Could not delete old test data")

	bh := New(dataDir)

	// Return a function to tear down the test
	return bh, func(t *testing.T) {
		bh.Close(context.Background())
		err = os.RemoveAll(dataDir)
		require.NoErrorf(t, err, "Could not delete old test data")
	}
}
