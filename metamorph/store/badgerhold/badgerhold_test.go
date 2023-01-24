package badgerhold

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

				var data2 *store.StoreData
				data2, err = bh.Get(context.Background(), hash)
				require.NoError(t, err)
				assert.Equal(t, hash, data2.Hash)

				err = bh.Del(context.Background(), hash)
				require.NoError(t, err)
			}
		}(workerId)
	}

	wg.Wait()

}

func TestGetUnseen(t *testing.T) {
	t.Run("no unseen", func(t *testing.T) {
		bh, tearDown := setupSuite(t)
		defer tearDown(t)

		tests.NoUnseen(t, bh)

	})

	t.Run("multiple unseen", func(t *testing.T) {
		bh, tearDown := setupSuite(t)
		defer tearDown(t)

		tests.MultipleUnseen(t, bh)
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

		tests.UpdateMined(t, bh)
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

		tests.UpdateStatus(t, bh)
	})

	t.Run("update status with error", func(t *testing.T) {
		bh, tearDown := setupSuite(t)
		defer tearDown(t)

		tests.UpdateStatusWithError(t, bh)
	})
}

func setupSuite(t *testing.T) (store.MetamorphStore, func(t *testing.T)) {
	dataDir := "./data-" + random.String(10)

	err := os.RemoveAll(dataDir)
	require.NoErrorf(t, err, "Could not delete old test data")

	var bh store.MetamorphStore
	bh, err = New(dataDir)
	require.NoErrorf(t, err, "could not init badgerhold store")

	// Return a function to tear down the test
	return bh, func(t *testing.T) {
		bh.Close(context.Background())
		err = os.RemoveAll(dataDir)
		require.NoErrorf(t, err, "Could not delete old test data")
	}
}

func TestBadger_GetBlockProcessed(t *testing.T) {
	bh, tearDown := setupSuite(t)
	defer tearDown(t)

	ctx := context.Background()

	timeNow := time.Now()
	err := bh.SetBlockProcessed(ctx, tests.Block1Bytes)
	require.NoError(t, err)

	testStruct := []struct {
		name      string
		store     *BadgerHold
		blockHash []byte
		want      *time.Time
		wantErr   assert.ErrorAssertionFunc
	}{
		{
			name:      "success",
			store:     bh.(*BadgerHold),
			blockHash: tests.Block1Bytes,
			want:      &timeNow,
			wantErr:   assert.NoError,
		},
		{
			name:      "missing",
			store:     bh.(*BadgerHold),
			blockHash: []byte("block hash"),
			want:      nil,
			wantErr:   assert.NoError,
		},
	}
	for _, tt := range testStruct {
		t.Run(tt.name, func(t *testing.T) {
			s := &BadgerHold{
				store: tt.store.store,
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
	bh, tearDown := setupSuite(t)
	defer tearDown(t)

	ctx := context.Background()
	testStructs := []struct {
		name      string
		store     *BadgerHold
		blockHash []byte
		wantErr   assert.ErrorAssertionFunc
	}{
		{
			name:      "success",
			store:     bh.(*BadgerHold),
			blockHash: tests.Block1Bytes,
			wantErr:   assert.NoError,
		},
	}
	for _, tt := range testStructs {
		t.Run(tt.name, func(t *testing.T) {
			s := &BadgerHold{
				store: tt.store.store,
			}
			tt.wantErr(t, s.SetBlockProcessed(ctx, tt.blockHash), fmt.Sprintf("SetBlockProcessed(%v)", tt.blockHash))
		})
	}
}
