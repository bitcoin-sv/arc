package sql

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/TAAL-GmbH/arc/metamorph/store/tests"
	"github.com/ordishs/go-utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGet(t *testing.T) {
	t.Run("get - error", func(t *testing.T) {
		sqliteDB, err := New("sqlite_memory")
		require.NoError(t, err)

		defer sqliteDB.Close(context.Background())

		_, err = sqliteDB.Get(context.Background(), []byte("hello world"))
		require.ErrorIs(t, store.ErrNotFound, err)
	})
}

func TestPutGetDelete(t *testing.T) {
	sqliteDB, err := New("sqlite_memory")
	require.NoError(t, err)

	defer sqliteDB.Close(context.Background())

	hash := utils.Sha256d([]byte("hello world"))

	data := &store.StoreData{
		Hash: hash,
	}

	err = sqliteDB.Set(context.Background(), hash, data)
	require.NoError(t, err)

	data2, err := sqliteDB.Get(context.Background(), hash)
	require.NoError(t, err)
	assert.Equal(t, data, data2)

	err = sqliteDB.Del(context.Background(), hash)
	require.NoError(t, err)
}

func TestPutGetMulti(t *testing.T) {
	sqliteDB, sqlErr := New("sqlite_memory")
	require.NoError(t, sqlErr)

	defer sqliteDB.Close(context.Background())

	var wg sync.WaitGroup

	for workerId := 0; workerId < 10; workerId++ {
		wg.Add(1)

		ctx := context.Background()
		go func(workerId int) {
			defer wg.Done()

			for i := 0; i < 100; i++ {
				data := []byte(fmt.Sprintf("Hello world %d-%d-%d", workerId, i, time.Now().UnixMilli()))

				hash := utils.Sha256d(data)

				err := sqliteDB.Set(ctx, hash, &store.StoreData{
					Hash: hash,
				})
				require.NoError(t, err)

				var data2 *store.StoreData
				data2, err = sqliteDB.Get(context.Background(), hash)
				require.NoError(t, err)
				assert.Equal(t, hash, data2.Hash)

				err = sqliteDB.Del(context.Background(), hash)
				require.NoError(t, err)
			}
		}(workerId)
	}

	wg.Wait()

}

func TestGetUnseen(t *testing.T) {
	t.Run("no unseen", func(t *testing.T) {
		sqliteDB, err := New("sqlite_memory")
		require.NoError(t, err)

		defer sqliteDB.Close(context.Background())

		tests.NoUnseen(t, sqliteDB)
	})

	t.Run("multiple unseen", func(t *testing.T) {
		sqliteDB, err := New("sqlite_memory")
		require.NoError(t, err)

		defer sqliteDB.Close(context.Background())

		tests.MultipleUnseen(t, sqliteDB)
	})
}

func TestUpdateMined(t *testing.T) {
	t.Run("update mined - not found", func(t *testing.T) {
		sqliteDB, err := New("sqlite_memory")
		require.NoError(t, err)

		defer sqliteDB.Close(context.Background())

		err = sqliteDB.UpdateMined(context.Background(), tests.Tx1Bytes, []byte("block hash"), 123)
		require.NoError(t, err) // an error is not thrown if not found
	})

	t.Run("update announced to mined", func(t *testing.T) {
		sqliteDB, err := New("sqlite_memory")
		require.NoError(t, err)

		defer sqliteDB.Close(context.Background())

		tests.UpdateMined(t, sqliteDB)
	})
}

func TestUpdateStatus(t *testing.T) {
	t.Run("update status - not found", func(t *testing.T) {
		sqliteDB, err := New("sqlite_memory")
		require.NoError(t, err)

		defer sqliteDB.Close(context.Background())

		err = sqliteDB.UpdateStatus(context.Background(), tests.Tx1Bytes, metamorph_api.Status_SENT_TO_NETWORK, "")
		require.NoError(t, err) // an error is not thrown if not found
	})

	t.Run("update status", func(t *testing.T) {
		sqliteDB, err := New("sqlite_memory")
		require.NoError(t, err)

		defer sqliteDB.Close(context.Background())
		tests.UpdateStatus(t, sqliteDB)
	})

	t.Run("update status with error", func(t *testing.T) {
		sqliteDB, err := New("sqlite_memory")
		require.NoError(t, err)

		defer sqliteDB.Close(context.Background())
		tests.UpdateStatusWithError(t, sqliteDB)
	})
}

func TestBadger_GetBlockProcessed(t *testing.T) {
	sqliteDB, err := New("sqlite_memory")
	require.NoError(t, err)

	defer sqliteDB.Close(context.Background())

	ctx := context.Background()

	timeNow := time.Now()
	err = sqliteDB.SetBlockProcessed(ctx, tests.Block1Bytes)
	require.NoError(t, err)

	testStruct := []struct {
		name      string
		store     *SQL
		blockHash []byte
		want      *time.Time
		wantErr   assert.ErrorAssertionFunc
	}{
		{
			name:      "success",
			store:     sqliteDB.(*SQL),
			blockHash: tests.Block1Bytes,
			want:      &timeNow,
			wantErr:   assert.NoError,
		},
		{
			name:      "missing",
			store:     sqliteDB.(*SQL),
			blockHash: []byte("block hash"),
			want:      nil,
			wantErr:   assert.NoError,
		},
	}
	for _, tt := range testStruct {
		t.Run(tt.name, func(t *testing.T) {
			s := &SQL{
				db: tt.store.db,
			}
			got, err := s.GetBlockProcessed(ctx, tt.blockHash)
			if !tt.wantErr(t, err, fmt.Sprintf("GetBlockProcessed(%v)", tt.blockHash)) {
				return
			}
			if tt.want == nil {
				assert.Nil(t, got, "GetBlockProcessed(%v)", tt.blockHash)
				return
			}
			assert.WithinDurationf(t, *tt.want, *got, 1000000, "GetBlockProcessed(%v)", tt.blockHash)
		})
	}
}

func TestBadger_SetBlockProcessed(t *testing.T) {
	sqliteDB, err := New("sqlite_memory")
	require.NoError(t, err)

	defer sqliteDB.Close(context.Background())

	ctx := context.Background()
	testStructs := []struct {
		name      string
		store     *SQL
		blockHash []byte
		wantErr   assert.ErrorAssertionFunc
	}{
		{
			name:      "success",
			store:     sqliteDB.(*SQL),
			blockHash: tests.Block1Bytes,
			wantErr:   assert.NoError,
		},
	}
	for _, tt := range testStructs {
		t.Run(tt.name, func(t *testing.T) {
			s := &SQL{
				db: tt.store.db,
			}
			tt.wantErr(t, s.SetBlockProcessed(ctx, tt.blockHash), fmt.Sprintf("SetBlockProcessed(%v)", tt.blockHash))
		})
	}
}
