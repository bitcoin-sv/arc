package badgerhold

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	pb "github.com/TAAL-GmbH/arc/metamorph/api"
	"github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/ordishs/go-utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSuite(t *testing.T) func(t *testing.T) {
	t.Log("setup suite")

	err := os.RemoveAll("./data")
	require.NoErrorf(t, err, "Could not delete old test data")

	// Return a function to tear down the test
	return func(t *testing.T) {
		t.Log("tear down suite")
		err = os.RemoveAll("./data")
		require.NoErrorf(t, err, "Could not delete old test data")
	}
}

func TestPutGetDelete(t *testing.T) {
	tearDown := setupSuite(t)
	defer tearDown(t)

	bh := New()

	defer bh.Close(context.Background())

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
	tearDown := setupSuite(t)
	defer tearDown(t)

	bh := New()

	defer bh.Close(context.Background())

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
		tearDown := setupSuite(t)
		defer tearDown(t)

		bh := New()

		hashes := [][]byte{
			utils.Sha256d([]byte("hello world")),
			utils.Sha256d([]byte("hello again")),
			utils.Sha256d([]byte("hello again again")),
		}

		for _, hash := range hashes {
			err := bh.Set(context.Background(), hash, &store.StoreData{
				Hash:   hash,
				Status: pb.Status_SEEN_ON_NETWORK,
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
		tearDown := setupSuite(t)
		defer tearDown(t)

		bh := New()

		hashes := [][]byte{
			utils.Sha256d([]byte("hello world")),
			utils.Sha256d([]byte("hello again")),
			utils.Sha256d([]byte("hello again again")),
		}

		for _, hash := range hashes {
			err := bh.Set(context.Background(), hash, &store.StoreData{
				Hash:   hash,
				Status: pb.Status_ANNOUNCED_TO_NETWORK,
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
