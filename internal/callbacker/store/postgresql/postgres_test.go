package postgresql

import (
	"context"
	"database/sql"
	"log"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/callbacker/store"
	testutils "github.com/bitcoin-sv/arc/internal/test_utils"
	"github.com/bitcoin-sv/arc/internal/testdata"
	"github.com/go-testfixtures/testfixtures/v3"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/require"

	_ "github.com/golang-migrate/migrate/v4/source/file"

	tutils "github.com/bitcoin-sv/arc/internal/callbacker/store/postgresql/internal/tests"
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

	port := "5433"
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

	now := time.Date(2023, 10, 1, 14, 25, 0, 0, time.UTC)

	postgresDB, err := New(dbInfo, 10, 10)
	require.NoError(t, err)
	defer postgresDB.Close()

	t.Run("set many", func(t *testing.T) {
		// given
		defer pruneTables(t, postgresDB.db)

		data := []*store.CallbackData{
			{
				Url:       "https://test-callback-1/",
				Token:     "token",
				TxID:      testdata.TX2,
				TxStatus:  "SEEN_ON_NETWORK",
				Timestamp: now,
			},
			{
				Url:         "https://test-callback-1/",
				Token:       "token",
				TxID:        testdata.TX2,
				TxStatus:    "MINED",
				Timestamp:   now,
				BlockHash:   ptrTo(testdata.Block1),
				BlockHeight: ptrTo(uint64(4524235)),
			},
			{
				// duplicate
				Url:         "https://test-callback-1/",
				Token:       "token",
				TxID:        testdata.TX2,
				TxStatus:    "MINED",
				Timestamp:   now,
				BlockHash:   &testdata.Block1,
				BlockHeight: ptrTo(uint64(4524235)),
			},
			{
				Url:          "https://test-callback-2/",
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
				Url:          "https://test-callback-2/",
				Token:        "token",
				TxID:         testdata.TX3,
				TxStatus:     "MINED",
				Timestamp:    now,
				BlockHash:    &testdata.Block2,
				BlockHeight:  ptrTo(uint64(4524236)),
				CompetingTxs: []string{testdata.TX2},
			},
			{
				Url:         "https://test-callback-2/",
				TxID:        testdata.TX2,
				TxStatus:    "MINED",
				Timestamp:   now,
				BlockHash:   &testdata.Block1,
				BlockHeight: ptrTo(uint64(4524235)),
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

	t.Run("pop many", func(t *testing.T) {
		// given
		defer pruneTables(t, postgresDB.db)
		loadFixtures(t, postgresDB.db, "fixtures/pop_many")

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

			callbacks := records.([]*store.CallbackData)
			require.Equal(t, popLimit, len(callbacks))
		}
	})

}

func pruneTables(t *testing.T, db *sql.DB) {
	t.Helper()

	_, err := db.Exec("TRUNCATE TABLE callbacker.callbacks;")
	if err != nil {
		t.Fatal(err)
	}
}

func loadFixtures(t *testing.T, db *sql.DB, path string) {
	t.Helper()

	fixtures, err := testfixtures.New(
		testfixtures.Database(db),
		testfixtures.Dialect("postgresql"),
		testfixtures.Directory(path), // The directory containing the YAML files
	)
	if err != nil {
		t.Fatalf("failed to create fixtures: %v", err)
	}

	err = fixtures.Load()
	if err != nil {
		t.Fatalf("failed to load fixtures: %v", err)
	}
}
