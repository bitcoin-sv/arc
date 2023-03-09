package memorystore

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/TAAL-GmbH/arc/metamorph/store/tests"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
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
		bh, err := New()
		require.NoError(t, err)
		defer bh.Close(context.Background())

		_, err = bh.Get(context.Background(), []byte("hello world"))
		require.ErrorIs(t, store.ErrNotFound, err)
	})
}

func TestPutGetDelete(t *testing.T) {
	bh, err := New()
	require.NoError(t, err)

	defer bh.Close(context.Background())

	hash, err := chainhash.NewHash(utils.Sha256d([]byte("hello world")))
	require.NoError(t, err)

	data := &store.StoreData{
		Hash: hash,
	}

	err = bh.Set(context.Background(), hash[:], data)
	require.NoError(t, err)

	data2, err := bh.Get(context.Background(), hash[:])
	require.NoError(t, err)
	assert.Equal(t, data, data2)

	err = bh.Del(context.Background(), hash[:])
	require.NoError(t, err)
}

func TestPutGetMulti(t *testing.T) {
	bh, err := New()
	require.NoError(t, err)

	defer bh.Close(context.Background())

	var wg sync.WaitGroup

	for workerId := 0; workerId < 100; workerId++ {
		wg.Add(1)

		go func(workerId int) {
			defer wg.Done()

			for i := 0; i < 100; i++ {
				data := []byte(fmt.Sprintf("Hello world %d-%d", workerId, i))

				hash, err := chainhash.NewHash(utils.Sha256d(data))
				require.NoError(t, err)

				err = bh.Set(context.Background(), hash[:], &store.StoreData{
					Hash: hash,
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
		bh, err := New()
		require.NoError(t, err)

		defer bh.Close(context.Background())

		tests.NoUnmined(t, bh)
	})

	t.Run("multiple unseen", func(t *testing.T) {
		bh, err := New()
		require.NoError(t, err)

		defer bh.Close(context.Background())

		tests.MultipleUnmined(t, bh)
	})
}

func TestUpdateMined(t *testing.T) {
	t.Run("update mined - not found", func(t *testing.T) {
		bh, err := New()
		require.NoError(t, err)

		defer bh.Close(context.Background())

		hash, err := chainhash.NewHash(tx1Bytes)
		require.NoError(t, err)

		err = bh.UpdateMined(context.Background(), hash, tests.Block1Hash, 123)
		require.NoError(t, err) // an error is not thrown if not found
	})

	t.Run("update announced to mined", func(t *testing.T) {
		bh, err := New()
		require.NoError(t, err)

		defer bh.Close(context.Background())

		tests.UpdateMined(t, bh)
	})
}

func TestUpdateStatus(t *testing.T) {
	t.Run("update status - not found", func(t *testing.T) {
		bh, err := New()
		require.NoError(t, err)

		defer bh.Close(context.Background())

		hash, err := chainhash.NewHash(tx1Bytes)
		require.NoError(t, err)
		err = bh.UpdateStatus(context.Background(), hash, metamorph_api.Status_SENT_TO_NETWORK, "")
		require.NoError(t, err) // an error is not thrown if not found
	})

	t.Run("update status", func(t *testing.T) {
		bh, err := New()
		require.NoError(t, err)

		defer bh.Close(context.Background())

		tests.UpdateStatus(t, bh)
	})

	t.Run("update status with error", func(t *testing.T) {
		bh, err := New()
		require.NoError(t, err)

		defer bh.Close(context.Background())

		tests.UpdateStatusWithError(t, bh)
	})
}
