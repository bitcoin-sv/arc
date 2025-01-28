package postgresql

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/callbacker/store"
	tutils "github.com/bitcoin-sv/arc/internal/callbacker/store/postgresql/internal/tests"
	testutils "github.com/bitcoin-sv/arc/internal/test_utils"
	"github.com/bitcoin-sv/arc/internal/testdata"
)

const (
	migrationsPath = "file://migrations"
)

var dbInfo string

func TestMain(m *testing.M) {
	os.Exit(testmain(m))
}

func testmain(m *testing.M) int {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Printf("failed to create pool: %v", err)
		return 1
	}

	port := "5436"
	resource, connStr, err := testutils.RunAndMigratePostgresql(pool, port, "callbacker", migrationsPath)
	if err != nil {
		log.Print(err)
		return 1
	}
	defer func() {
		err = pool.Purge(resource)
		if err != nil {
			log.Fatalf("failed to purge pool: %v", err)
		}
	}()

	dbInfo = connStr
	return m.Run()
}

func TestPostgresDBt(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	now := time.Date(2024, 9, 1, 12, 25, 0, 0, time.UTC)

	postgresDB, err := New(dbInfo, 10, 10)
	require.NoError(t, err)
	defer postgresDB.Close()

	t.Run("set many", func(t *testing.T) {
		// given
		defer pruneTables(t, postgresDB.db)

		data := []*store.CallbackData{
			{
				URL:       "https://test-callback-1/",
				Token:     "token",
				TxID:      testdata.TX2,
				TxStatus:  "SEEN_ON_NETWORK",
				Timestamp: now,
			},
			{
				URL:         "https://test-callback-1/",
				Token:       "token",
				TxID:        testdata.TX2,
				TxStatus:    "MINED",
				Timestamp:   now,
				BlockHash:   ptrTo(testdata.Block1),
				BlockHeight: ptrTo(uint64(4524235)),
			},
			{
				// duplicate
				URL:         "https://test-callback-1/",
				Token:       "token",
				TxID:        testdata.TX2,
				TxStatus:    "MINED",
				Timestamp:   now,
				BlockHash:   &testdata.Block1,
				BlockHeight: ptrTo(uint64(4524235)),
			},
			{
				URL:          "https://test-callback-2/",
				Token:        "token",
				TxID:         testdata.TX3,
				TxStatus:     "MINED",
				Timestamp:    now,
				BlockHash:    &testdata.Block2,
				BlockHeight:  ptrTo(uint64(4524236)),
				CompetingTxs: []string{testdata.TX2},
			},
			{
				// duplicate
				URL:          "https://test-callback-2/",
				Token:        "token",
				TxID:         testdata.TX3,
				TxStatus:     "MINED",
				Timestamp:    now,
				BlockHash:    &testdata.Block2,
				BlockHeight:  ptrTo(uint64(4524236)),
				CompetingTxs: []string{testdata.TX2},
			},
			{
				URL:         "https://test-callback-2/",
				TxID:        testdata.TX2,
				TxStatus:    "MINED",
				Timestamp:   now,
				BlockHash:   &testdata.Block1,
				BlockHeight: ptrTo(uint64(4524235)),
			},
			{
				URL:            "https://test-callback-3/",
				TxID:           testdata.TX2,
				TxStatus:       "MINED",
				Timestamp:      now,
				BlockHash:      &testdata.Block1,
				BlockHeight:    ptrTo(uint64(4524235)),
				PostponedUntil: ptrTo(now.Add(10 * time.Minute)),
			},

			{
				URL:            "https://test-callback-3/",
				TxID:           testdata.TX3,
				TxStatus:       "MINED",
				Timestamp:      now,
				BlockHash:      &testdata.Block1,
				BlockHeight:    ptrTo(uint64(4524235)),
				PostponedUntil: ptrTo(now.Add(10 * time.Minute)),
			},
		}

		// when
		err = postgresDB.SetMany(context.Background(), data)

		// then
		require.NoError(t, err)

		// read all from db
		dbCallbacks := tutils.ReadAllCallbacks(t, postgresDB.db)
		for _, c := range dbCallbacks {
			found := false
			for i, ur := range data {
				if ur == nil {
					continue
				}

				if tutils.CallbackRecordEqual(ur, c) {
					// remove if found
					data[i] = nil
					found = true
					break
				}
			}

			require.True(t, found)
		}
	})

	t.Run("get and delete", func(t *testing.T) {
		// given
		defer pruneTables(t, postgresDB.db)
		testutils.LoadFixtures(t, postgresDB.db, "fixtures/pop_many")

		ctx := context.Background()
		var rowsBefore int
		err = postgresDB.db.QueryRowContext(ctx,
			"SELECT count(*) FROM callbacker.callbacks WHERE url=$1",
			"https://arc-callback-2/callback").Scan(&rowsBefore)
		require.NoError(t, err)

		require.Equal(t, 28, rowsBefore)

		const popLimit = 10
		records, err := postgresDB.GetAndDelete(ctx, "https://arc-callback-2/callback", popLimit)
		require.NoError(t, err)

		require.Len(t, records, popLimit)

		var rowsAfter int
		err = postgresDB.db.QueryRowContext(ctx,
			"SELECT count(*) FROM callbacker.callbacks WHERE url=$1",
			"https://arc-callback-2/callback").Scan(&rowsAfter)
		require.NoError(t, err)
		require.Equal(t, 18, rowsAfter)
	})

	t.Run("pop many", func(t *testing.T) {
		// given
		defer pruneTables(t, postgresDB.db)
		testutils.LoadFixtures(t, postgresDB.db, "fixtures/pop_many")

		const concurentCalls = 5
		const popLimit = 10

		// count current records
		count := tutils.CountCallbacks(t, postgresDB.db)
		require.GreaterOrEqual(t, count, concurentCalls*popLimit)

		ctx := context.Background()
		start := make(chan struct{})
		rm := sync.Map{}
		wg := sync.WaitGroup{}

		// when
		wg.Add(concurentCalls)
		for i := range concurentCalls {
			go func() {
				defer wg.Done()
				<-start

				records, err := postgresDB.PopMany(ctx, popLimit)
				require.NoError(t, err)

				rm.Store(i, records)
			}()
		}

		close(start) // signal all goroutines to start
		wg.Wait()

		// then
		count2 := tutils.CountCallbacks(t, postgresDB.db)
		require.Equal(t, count-concurentCalls*popLimit, count2)

		for i := range concurentCalls {
			records, ok := rm.Load(i)
			require.True(t, ok)

			callbacks, ok := records.([]*store.CallbackData)
			require.True(t, ok)
			require.Equal(t, popLimit, len(callbacks))
		}
	})

	t.Run("pop failed many", func(t *testing.T) {
		// given
		defer pruneTables(t, postgresDB.db)
		testutils.LoadFixtures(t, postgresDB.db, "fixtures/pop_failed_many")

		const concurentCalls = 5
		const popLimit = 10

		// count current records
		countAll := tutils.CountCallbacks(t, postgresDB.db)
		require.GreaterOrEqual(t, countAll, concurentCalls*popLimit)
		countToPop := tutils.CountCallbacksWhere(t, postgresDB.db, fmt.Sprintf("postponed_until <= '%s'", now.Format(time.RFC3339)))
		require.Greater(t, countToPop, popLimit)

		ctx := context.Background()
		start := make(chan struct{})
		rm := sync.Map{}
		wg := sync.WaitGroup{}

		// when
		wg.Add(concurentCalls)
		for i := range concurentCalls {
			go func() {
				defer wg.Done()
				<-start

				records, err := postgresDB.PopFailedMany(ctx, now, popLimit)
				require.NoError(t, err)

				rm.Store(i, records)
			}()
		}

		close(start) // signal all goroutines to start
		wg.Wait()

		// then
		count2 := tutils.CountCallbacks(t, postgresDB.db)
		require.Equal(t, countAll-countToPop, count2)

		for i := range concurentCalls {
			records, ok := rm.Load(i)
			require.True(t, ok)

			callbacks, ok := records.([]*store.CallbackData)
			require.True(t, ok)
			require.LessOrEqual(t, len(callbacks), popLimit)
		}
	})

	t.Run("delete failed older than", func(t *testing.T) {
		// given
		defer pruneTables(t, postgresDB.db)
		testutils.LoadFixtures(t, postgresDB.db, "fixtures/delete_failed_older_than")

		const concurentCalls = 5

		// count current records
		countAll := tutils.CountCallbacks(t, postgresDB.db)
		countToDelete := tutils.CountCallbacksWhere(t, postgresDB.db, fmt.Sprintf("timestamp <= '%s' AND postponed_until IS NOT NULL", now.Format(time.RFC3339)))

		ctx := context.Background()
		start := make(chan struct{})
		wg := sync.WaitGroup{}

		// when
		wg.Add(concurentCalls)
		for range concurentCalls {
			go func() {
				defer wg.Done()
				<-start

				err := postgresDB.DeleteFailedOlderThan(ctx, now)
				require.NoError(t, err)
			}()
		}

		close(start) // signal all goroutines to start
		wg.Wait()

		// then
		require.Equal(t, countAll-countToDelete, tutils.CountCallbacks(t, postgresDB.db))
	})

	t.Run("set URL mapping", func(t *testing.T) {
		// given
		defer pruneTables(t, postgresDB.db)

		// when
		ctx := context.Background()
		err = postgresDB.SetURLMapping(ctx, store.URLMapping{
			URL:      "https://callback-receiver.com/",
			Instance: "host1",
		})

		// then
		require.NoError(t, err)

		// when
		err = postgresDB.SetURLMapping(ctx, store.URLMapping{
			URL:      "https://callback-receiver.com/",
			Instance: "host2",
		})

		// then
		require.ErrorIs(t, err, store.ErrURLMappingDuplicateKey)
	})

	t.Run("delete URL mapping", func(t *testing.T) {
		// given
		defer pruneTables(t, postgresDB.db)

		ctx := context.Background()
		err = postgresDB.SetURLMapping(ctx, store.URLMapping{
			URL:      "https://callback-receiver.com/",
			Instance: "host1",
		})
		require.NoError(t, err)

		// when
		err = postgresDB.DeleteURLMapping(ctx, "host1")
		require.NoError(t, err)

		// then
		mappings, err := postgresDB.GetURLMappings(ctx)
		require.NoError(t, err)

		require.Len(t, mappings, 0)
	})

	t.Run("get URL mappings", func(t *testing.T) {
		// given
		defer pruneTables(t, postgresDB.db)

		ctx := context.Background()
		err = postgresDB.SetURLMapping(ctx, store.URLMapping{
			URL:      "https://callback-receiver.com/1",
			Instance: "host1",
		})
		require.NoError(t, err)
		err = postgresDB.SetURLMapping(ctx, store.URLMapping{
			URL:      "https://callback-receiver.com/2",
			Instance: "host2",
		})
		require.NoError(t, err)
		err = postgresDB.SetURLMapping(ctx, store.URLMapping{
			URL:      "https://callback-receiver.com/3",
			Instance: "host3",
		})
		require.NoError(t, err)
		err = postgresDB.SetURLMapping(ctx, store.URLMapping{
			URL:      "https://callback-receiver.com/4",
			Instance: "host4",
		})
		require.NoError(t, err)

		// when
		actual, err := postgresDB.GetURLMappings(ctx)

		// then
		require.NoError(t, err)
		expected := map[string]string{
			"https://callback-receiver.com/1": "host1",
			"https://callback-receiver.com/2": "host2",
			"https://callback-receiver.com/3": "host3",
			"https://callback-receiver.com/4": "host4",
		}
		require.True(t, reflect.DeepEqual(expected, actual))
	})
}

func pruneTables(t *testing.T, db *sql.DB) {
	testutils.PruneTables(t, db, "callbacker.callbacks")
	testutils.PruneTables(t, db, "callbacker.url_mapping")
}
