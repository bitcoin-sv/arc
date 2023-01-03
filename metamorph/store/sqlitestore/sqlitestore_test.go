package sqlitestore

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/TAAL-GmbH/arc/metamorph/store/tests"
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
		sqliteDB, closeDB := getTestDB(t)
		defer closeDB()

		_, err := sqliteDB.Get(context.Background(), []byte("hello world"))
		require.ErrorIs(t, store.ErrNotFound, err)
	})
}

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

	for workerId := 0; workerId < 10; workerId++ {
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

		tests.NoUnseen(t, sqliteDB)
	})

	t.Run("multiple unseen", func(t *testing.T) {
		sqliteDB, closeDB := getTestDB(t)
		defer closeDB()

		tests.MultipleUnseen(t, sqliteDB)
	})
}

func TestUpdateMined(t *testing.T) {
	t.Run("update mined - not found", func(t *testing.T) {
		sqliteDB, closeDB := getTestDB(t)
		defer closeDB()

		err := sqliteDB.UpdateMined(context.Background(), tx1Bytes, []byte("block hash"), 123)
		require.NoError(t, err) // an error is not thrown if not found
	})

	t.Run("update announced to mined", func(t *testing.T) {
		sqliteDB, closeDB := getTestDB(t)
		defer closeDB()

		tests.UpdateMined(t, sqliteDB)
	})
}

func TestUpdateStatus(t *testing.T) {
	t.Run("update status - not found", func(t *testing.T) {
		sqliteDB, closeDB := getTestDB(t)
		defer closeDB()

		err := sqliteDB.UpdateStatus(context.Background(), tx1Bytes, metamorph_api.Status_SENT_TO_NETWORK, "")
		require.NoError(t, err) // an error is not thrown if not found
	})

	t.Run("update status", func(t *testing.T) {
		sqliteDB, closeDB := getTestDB(t)
		defer closeDB()
		tests.UpdateStatus(t, sqliteDB)
	})

	t.Run("update status with error", func(t *testing.T) {
		sqliteDB, closeDB := getTestDB(t)
		defer closeDB()
		tests.UpdateStatusWithError(t, sqliteDB)
	})
}

func getTestDB(t *testing.T) (store.Store, func()) {

	dbLocation := "./test_" + random.String(10) + ".db"

	sqliteDB, err := New(dbLocation)
	require.NoError(t, err)

	return sqliteDB, func() {
		_ = sqliteDB.Close(context.Background())
		_ = os.Remove(dbLocation)
	}
}
