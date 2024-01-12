package sqlite

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/metamorph/store"
	"github.com/bitcoin-sv/arc/metamorph/store/tests"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGet(t *testing.T) {
	t.Run("get - error", func(t *testing.T) {
		sqliteDB, err := New(true, "")
		require.NoError(t, err)

		defer sqliteDB.Close(context.Background())

		_, err = sqliteDB.Get(context.Background(), []byte("hello world"))
		require.ErrorIs(t, store.ErrNotFound, err)
	})
}

func TestPutGetDelete(t *testing.T) {
	sqliteDB, err := New(true, "")
	require.NoError(t, err)

	defer sqliteDB.Close(context.Background())

	hash := chainhash.DoubleHashH([]byte("hello world"))

	data := &store.StoreData{
		Hash:     &hash,
		StoredAt: time.Now(),
	}

	err = sqliteDB.Set(context.Background(), hash[:], data)
	require.NoError(t, err)

	data2, err := sqliteDB.Get(context.Background(), hash[:])
	require.NoError(t, err)

	assert.WithinDurationf(t, data.StoredAt, data2.StoredAt, time.Second, "StoredAt should be within a second of each other")

	// these will be slightly off due to setting the time when inserting
	data.StoredAt = data2.StoredAt
	assert.Equal(t, data, data2)

	err = sqliteDB.Del(context.Background(), hash[:])
	require.NoError(t, err)
}

func TestPutGetMulti(t *testing.T) {
	sqliteDB, sqlErr := New(true, "")
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

				hash := chainhash.DoubleHashH(data)

				err := sqliteDB.Set(ctx, hash[:], &store.StoreData{
					Hash: &hash,
				})
				require.NoError(t, err)

				var data2 *store.StoreData
				data2, err = sqliteDB.Get(context.Background(), hash[:])
				require.NoError(t, err)
				assert.Equal(t, &hash, data2.Hash)

				err = sqliteDB.Del(context.Background(), hash[:])
				require.NoError(t, err)
			}
		}(workerId)
	}

	wg.Wait()
}

func TestGetUnmined(t *testing.T) {
	t.Run("no unseen", func(t *testing.T) {
		sqliteDB, err := New(true, "")
		require.NoError(t, err)

		defer sqliteDB.Close(context.Background())

		tests.NoUnmined(t, sqliteDB)
	})

	t.Run("multiple unmined", func(t *testing.T) {
		sqliteDB, err := New(true, "")
		require.NoError(t, err)

		defer sqliteDB.Close(context.Background())

		tests.MultipleUnmined(t, sqliteDB)
	})
}

func TestUpdateMined(t *testing.T) {
	t.Run("update mined - not found", func(t *testing.T) {
		sqliteDB, err := New(true, "")
		require.NoError(t, err)

		defer sqliteDB.Close(context.Background())

		err = sqliteDB.UpdateMined(context.Background(), tests.Tx1Hash, tests.Block1Hash, 123)
		require.NoError(t, err) // an error is not thrown if not found
	})

	t.Run("update announced to mined", func(t *testing.T) {
		sqliteDB, err := New(true, "")
		require.NoError(t, err)

		defer sqliteDB.Close(context.Background())

		tests.UpdateMined(t, sqliteDB)
	})
}

func TestUpdateStatus(t *testing.T) {
	t.Run("update status - not found", func(t *testing.T) {
		sqliteDB, err := New(true, "")
		require.NoError(t, err)

		defer sqliteDB.Close(context.Background())

		err = sqliteDB.UpdateStatus(context.Background(), tests.Tx1Hash, metamorph_api.Status_SENT_TO_NETWORK, "")
		require.ErrorIs(t, err, store.ErrNotFound) // an error is thrown if not found
	})

	t.Run("update status", func(t *testing.T) {
		sqliteDB, err := New(true, "")
		require.NoError(t, err)

		defer sqliteDB.Close(context.Background())
		tests.UpdateStatus(t, sqliteDB)
	})

	t.Run("update status with error", func(t *testing.T) {
		sqliteDB, err := New(true, "")
		require.NoError(t, err)

		defer sqliteDB.Close(context.Background())
		tests.UpdateStatusWithError(t, sqliteDB)
	})
}

func TestSQLite_GetBlockProcessed(t *testing.T) {
	now := time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC)
	sqliteDB, err := New(true, "", WithNow(func() time.Time {
		return now
	}))
	require.NoError(t, err)

	defer sqliteDB.Close(context.Background())

	ctx := context.Background()

	err = sqliteDB.SetBlockProcessed(ctx, tests.Block1Hash)
	require.NoError(t, err)

	testStruct := []struct {
		name      string
		blockHash *chainhash.Hash
		want      *time.Time
		wantErr   assert.ErrorAssertionFunc
	}{
		{
			name:      "success",
			blockHash: tests.Block1Hash,
			want:      &now,
			wantErr:   assert.NoError,
		},
		{
			name:      "missing",
			blockHash: tests.Block2Hash,
			want:      nil,
			wantErr:   assert.NoError,
		},
	}
	for _, tt := range testStruct {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sqliteDB.GetBlockProcessed(ctx, tt.blockHash)
			if !tt.wantErr(t, err, fmt.Sprintf("GetBlockProcessed(%v)", tt.blockHash)) {
				return
			}

			if tt.want == nil {
				assert.Nil(t, got, "GetBlockProcessed(%v)", tt.blockHash)
				return
			}

			require.Equal(t, tt.want, got)
		})
	}
}

func TestSQLite_SetBlockProcessed(t *testing.T) {
	sqliteDB, err := New(true, "")
	require.NoError(t, err)

	defer sqliteDB.Close(context.Background())

	ctx := context.Background()
	testStructs := []struct {
		name      string
		blockHash *chainhash.Hash
		wantErr   assert.ErrorAssertionFunc
	}{
		{
			name:      "success",
			blockHash: tests.Block1Hash,
			wantErr:   assert.NoError,
		},
	}
	for _, tt := range testStructs {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantErr(t, sqliteDB.SetBlockProcessed(ctx, tt.blockHash), fmt.Sprintf("SetBlockProcessed(%v)", tt.blockHash))
		})
	}
}
