package sqlitestore

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	pb "github.com/TAAL-GmbH/arc/metamorph/api"
	"github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/ordishs/go-utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPutGetDelete(t *testing.T) {
	sqliteDB, closeDB := getTestDB(t)
	defer closeDB()

	hash := utils.Sha256d([]byte("hello world"))

	data := &store.StoreData{
		Hash: hash,
	}

	err := sqliteDB.Set(context.Background(), hash, data)
	require.NoError(t, err)

	data2, err := sqliteDB.Get(context.Background(), hash)
	require.NoError(t, err)
	assert.Equal(t, data, data2)

	err = sqliteDB.Del(context.Background(), hash)
	require.NoError(t, err)
}

func TestPutGetMulti(t *testing.T) {
	sqliteDB, closeDB := getTestDB(t)
	defer closeDB()

	var wg sync.WaitGroup

	for workerId := 0; workerId < 100; workerId++ {
		wg.Add(1)

		go func(workerId int) {
			defer wg.Done()

			for i := 0; i < 100; i++ {
				data := []byte(fmt.Sprintf("Hello world %d-%d-%d", workerId, i, time.Now().UnixMilli()))

				hash := utils.Sha256d(data)

				err := sqliteDB.Set(context.Background(), hash, &store.StoreData{
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
		sqliteDB, closeDB := getTestDB(t)
		defer closeDB()

		defer sqliteDB.Close(context.Background())

		hashes := [][]byte{
			utils.Sha256d([]byte("hello world")),
			utils.Sha256d([]byte("hello again")),
			utils.Sha256d([]byte("hello again again")),
		}

		for _, hash := range hashes {
			err := sqliteDB.Set(context.Background(), hash, &store.StoreData{
				Hash:   hash,
				Status: pb.Status_SEEN_ON_NETWORK,
			})
			require.NoError(t, err)
		}

		unseen := make([]*store.StoreData, 0)
		err := sqliteDB.GetUnseen(context.Background(), func(s *store.StoreData) {
			unseen = append(unseen, s)
		})
		require.NoError(t, err)
		assert.Equal(t, 0, len(unseen))

		for _, hash := range hashes {
			err = sqliteDB.Del(context.Background(), hash)
			require.NoError(t, err)
		}
	})

	t.Run("multiple unseen", func(t *testing.T) {
		sqliteDB, closeDB := getTestDB(t)
		defer closeDB()

		hashes := [][]byte{
			utils.Sha256d([]byte("hello world")),
			utils.Sha256d([]byte("hello again")),
			utils.Sha256d([]byte("hello again again")),
		}

		for _, hash := range hashes {
			err := sqliteDB.Set(context.Background(), hash, &store.StoreData{
				Hash:   hash,
				Status: pb.Status_ANNOUNCED_TO_NETWORK,
			})
			require.NoError(t, err)
		}

		unseen := make([]*store.StoreData, 0)
		err := sqliteDB.GetUnseen(context.Background(), func(s *store.StoreData) {
			unseen = append(unseen, s)
		})
		require.NoError(t, err)
		assert.Equal(t, 3, len(unseen))

		for _, hash := range hashes {
			err = sqliteDB.Del(context.Background(), hash)
			require.NoError(t, err)
		}
	})
}

func getTestDB(t *testing.T) (store.Store, func()) {

	dbLocation := "./test_" + randomString(10) + ".db"

	sqliteDB, err := New(dbLocation)
	require.NoError(t, err)

	return sqliteDB, func() {
		_ = sqliteDB.Close(context.Background())
		_ = os.Remove(dbLocation)
	}
}

func randomString(length int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, length)
	rand.Read(b)
	return fmt.Sprintf("%x", b)[:length]
}
