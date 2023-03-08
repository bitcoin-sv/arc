package badger

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
	"github.com/dgraph-io/badger/v3"
	"github.com/labstack/gommon/random"
	"github.com/libsv/go-p2p/chaincfg/chainhash"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	hash := chainhash.DoubleHashH(data)

	storeData := &store.StoreData{
		Hash:  &hash,
		RawTx: data,
	}

	err := bh.Set(context.Background(), hash[:], storeData)
	require.NoError(t, err)

	data2, err := bh.Get(context.Background(), hash[:])
	require.NoError(t, err)
	assert.Equal(t, &hash, data2.Hash)
	assert.Equal(t, data, data2.RawTx)

	err = bh.Del(context.Background(), hash[:])
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

				hash := chainhash.DoubleHashH(data)

				err := bh.Set(context.Background(), hash[:], &store.StoreData{
					Hash:  &hash,
					RawTx: data,
				})
				require.NoError(t, err)

				var data2 *store.StoreData
				data2, err = bh.Get(context.Background(), hash[:])
				require.NoError(t, err)
				assert.Equal(t, &hash, data2.Hash)

				err = bh.Del(context.Background(), hash[:])
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

		tests.NoUnmined(t, bh)
	})

	t.Run("multiple unseen", func(t *testing.T) {
		bh, tearDown := setupSuite(t)
		defer tearDown(t)

		tests.MultipleUnmined(t, bh)
	})
}
func TestUpdateMined(t *testing.T) {
	t.Run("update mined - not found", func(t *testing.T) {
		bh, tearDown := setupSuite(t)
		defer tearDown(t)

		err := bh.UpdateMined(context.Background(), tests.Tx1Hash, tests.Block1Hash, 123)
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

		err := bh.UpdateStatus(context.Background(), tests.Tx1Hash, metamorph_api.Status_SENT_TO_NETWORK, "")
		require.ErrorIs(t, err, store.ErrNotFound) // an error is not thrown if not found
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
	err := bh.SetBlockProcessed(ctx, tests.Block1Hash)
	require.NoError(t, err)

	testStruct := []struct {
		name      string
		store     *badger.DB
		blockHash *chainhash.Hash
		want      *time.Time
		wantErr   assert.ErrorAssertionFunc
	}{
		{
			name:      "success",
			store:     bh.(*Badger).store,
			blockHash: tests.Block1Hash,
			want:      &timeNow,
			wantErr:   assert.NoError,
		},
		{
			name:      "missing",
			store:     bh.(*Badger).store,
			blockHash: tests.Block2Hash,
			want:      nil,
			wantErr:   assert.NoError,
		},
	}
	for _, tt := range testStruct {
		t.Run(tt.name, func(t *testing.T) {
			s := &Badger{
				store: tt.store,
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
		store     *badger.DB
		blockHash *chainhash.Hash
		wantErr   assert.ErrorAssertionFunc
	}{
		{
			name:      "success",
			store:     bh.(*Badger).store,
			blockHash: tests.Block1Hash,
			wantErr:   assert.NoError,
		},
	}
	for _, tt := range testStructs {
		t.Run(tt.name, func(t *testing.T) {
			s := &Badger{
				store: tt.store,
			}
			tt.wantErr(t, s.SetBlockProcessed(ctx, tt.blockHash), fmt.Sprintf("SetBlockProcessed(%v)", tt.blockHash))
		})
	}
}
