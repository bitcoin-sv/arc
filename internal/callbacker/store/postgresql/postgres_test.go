package postgresql

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ccoveille/go-safecast"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/callbacker/store"
	"github.com/bitcoin-sv/arc/internal/testdata"
	testutils "github.com/bitcoin-sv/arc/pkg/test_utils"
)

const (
	migrationsPath = "file://migrations"
)

var dbInfo string

func TestMain(m *testing.M) {
	flag.Parse()

	if testing.Short() {
		return
	}

	testmain(m)
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

	postgresDB, err := New(dbInfo, 10, 10, WithNow(func() time.Time { return now }))
	require.NoError(t, err)
	defer postgresDB.Close()

	t.Run("set many", func(t *testing.T) {
		// given
		defer pruneTables(t, postgresDB.db)
		testutils.LoadFixtures(t, postgresDB.db, "fixtures/set_many")

		cbData1 := &store.CallbackData{
			URL:       "https://test-callback-1/",
			Token:     "token",
			TxID:      testdata.TX2,
			TxStatus:  "SEEN_ON_NETWORK",
			Timestamp: now,
		}
		cbData2 := &store.CallbackData{
			URL:         "https://test-callback-1/",
			Token:       "token",
			TxID:        testdata.TX2,
			TxStatus:    "MINED",
			Timestamp:   now,
			BlockHash:   ptrTo(testdata.Block1),
			BlockHeight: ptrTo(uint64(4524235)),
		}

		cbData3 := &store.CallbackData{
			URL:          "https://test-callback-2/",
			Token:        "token",
			TxID:         testdata.TX3,
			TxStatus:     "MINED",
			Timestamp:    now,
			BlockHash:    &testdata.Block2,
			BlockHeight:  ptrTo(uint64(4524236)),
			CompetingTxs: []string{testdata.TX2},
		}

		cbData4 := &store.CallbackData{
			URL:         "https://arc-callback-1/callback",
			Token:       "token",
			TxID:        "96cbf8ba96dc3bad6ecc19ce34d1edbf57b2bc6f76cc3d80efdca95599cf5c28",
			TxStatus:    "MINED",
			Timestamp:   now,
			BlockHash:   &testdata.Block1,
			BlockHeight: ptrTo(uint64(4524235)),
		}

		data := []*store.CallbackData{
			cbData1,
			cbData2,
			cbData2, // duplicate
			cbData3,
			cbData3, // duplicate
			cbData4,
		}

		// when
		err = postgresDB.SetMany(context.Background(), data)

		// then
		require.NoError(t, err)

		dbCallbacks := readAllCallbacks(t, postgresDB.db)
		require.NoError(t, err)

		expected := []*store.CallbackData{
			cbData1,
			cbData2,
			cbData3,
			cbData4,
		}

		for _, c := range dbCallbacks {
			found := false
			for i, ur := range expected {
				if reflect.DeepEqual(ur, c) {
					// remove if found
					data[i] = nil
					found = true
					break
				}
			}

			require.True(t, found)
		}
	})

	t.Run("get and mark sent", func(t *testing.T) {
		// given
		defer pruneTables(t, postgresDB.db)
		testutils.LoadFixtures(t, postgresDB.db, "fixtures/get_and_mark_sent")
		qCount := "SELECT count(*) FROM callbacker.callbacks WHERE url=$1"
		ctx := context.Background()
		var rowsCount int
		err = postgresDB.db.QueryRowContext(ctx, qCount, "https://arc-callback-2/callback").Scan(&rowsCount)
		require.NoError(t, err)

		require.Equal(t, 10, rowsCount)

		const limit = 10
		records, _, rollback, err := postgresDB.GetAndMarkSent(ctx, "https://arc-callback-2/callback", limit, 100*time.Hour, false)
		require.NoError(t, err)
		err = rollback()
		require.NoError(t, err)
		require.Len(t, records, 8)
		err = postgresDB.db.QueryRowContext(ctx, qCount, "https://arc-callback-2/callback").Scan(&rowsCount)
		require.NoError(t, err)
		require.Equal(t, 10, rowsCount)

		records, commit, _, err := postgresDB.GetAndMarkSent(ctx, "https://arc-callback-2/callback", limit, 100*time.Hour, false)
		require.NoError(t, err)
		err = commit()
		require.NoError(t, err)
		require.Len(t, records, 8)
		err = postgresDB.db.QueryRowContext(ctx, qCount, "https://arc-callback-2/callback").Scan(&rowsCount)
		require.NoError(t, err)
		require.Equal(t, 10, rowsCount)

		ids := make([]int64, len(records))
		for i, record := range records {
			ids[i] = record.ID
		}

		db, err := sqlx.Open("postgres", dbInfo)
		require.NoError(t, err)

		var sentAtDates []time.Time
		const q = `SELECT sent_at FROM callbacker.callbacks WHERE id = ANY($1::INTEGER[]) `
		err = db.Select(&sentAtDates, q, pq.Array(ids))
		require.NoError(t, err)

		for _, sentAtDate := range sentAtDates {
			require.Equal(t, now.UTC(), sentAtDate.UTC())
		}
	})

	t.Run("set sent", func(t *testing.T) {
		defer pruneTables(t, postgresDB.db)
		testutils.LoadFixtures(t, postgresDB.db, "fixtures/set_sent")
		ctx := context.Background()

		err = postgresDB.SetSent(ctx, []int64{1, 2, 3})
		require.NoError(t, err)

		r, err := postgresDB.db.QueryContext(ctx,
			`SELECT sent_at, pending FROM callbacker.callbacks WHERE id = ANY($1::INTEGER[])`,
			pq.Array([]int64{1, 2, 3}),
		)
		require.NoError(t, err)

		for r.Next() {
			var sentAt sql.NullTime
			var pending sql.NullTime
			err = r.Scan(&sentAt, &pending)
			require.NoError(t, err)
			require.True(t, sentAt.Valid)
			require.Equal(t, now.UTC(), sentAt.Time.UTC())
			require.False(t, pending.Valid)
		}
	})

	t.Run("get many", func(t *testing.T) {
		// given
		defer pruneTables(t, postgresDB.db)
		testutils.LoadFixtures(t, postgresDB.db, "fixtures/get_many")
		ctx := context.Background()

		const limit = 20
		records, err := postgresDB.GetMany(ctx, limit, 1*time.Hour, false)
		require.NoError(t, err)
		require.Len(t, records, 6)

		ids := make([]int64, len(records))
		for i, record := range records {
			require.Equal(t, "https://arc-callback-2/callback", record.URL)
			ids[i] = record.ID
		}

		r, err := postgresDB.db.QueryContext(ctx,
			`SELECT pending FROM callbacker.callbacks WHERE id = ANY($1::INTEGER[])`,
			pq.Array(ids),
		)
		defer r.Close()
		require.NoError(t, err)

		for r.Next() {
			var pending time.Time
			err = r.Scan(&pending)
			require.NoError(t, err)
			require.Equal(t, now.UTC(), pending.UTC())
		}
	})

	t.Run("delete older than", func(t *testing.T) {
		// given
		defer pruneTables(t, postgresDB.db)
		testutils.LoadFixtures(t, postgresDB.db, "fixtures/delete_older_than")
		ctx := context.Background()

		var rowsBefore int
		err = postgresDB.db.QueryRowContext(ctx, "SELECT COUNT(1) FROM callbacker.callbacks").Scan(&rowsBefore)
		require.NoError(t, err)
		require.Equal(t, 30, rowsBefore)

		err := postgresDB.DeleteOlderThan(ctx, now)
		require.NoError(t, err)

		var rowsAfter int
		err = postgresDB.db.QueryRowContext(ctx, "SELECT COUNT(1) FROM callbacker.callbacks").Scan(&rowsAfter)
		require.NoError(t, err)

		// then
		require.Equal(t, 3, rowsAfter)
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
		rowsAffected, err := postgresDB.DeleteURLMapping(ctx, "host1")
		require.NoError(t, err)
		require.Equal(t, int64(1), rowsAffected)

		// then
		mappings, err := postgresDB.GetURLMappings(ctx)
		require.NoError(t, err)

		require.Len(t, mappings, 0)
	})

	t.Run("delete URL mappings except", func(t *testing.T) {
		// given
		defer pruneTables(t, postgresDB.db)
		testutils.LoadFixtures(t, postgresDB.db, "fixtures/delete_url_mappings_except")
		ctx := context.Background()

		// when
		rowsAffected, err := postgresDB.DeleteURLMappingsExcept(ctx, []string{"host3", "host5"})
		require.NoError(t, err)
		require.Equal(t, int64(5), rowsAffected)

		// then
		mappings, err := postgresDB.GetURLMappings(ctx)
		require.NoError(t, err)
		expectedMappings := map[string]string{
			"https://abc3.com/callback": "host3",
			"https://abc4.com/callback": "host3",
			"https://abc8.com/callback": "host5",
		}

		require.Equal(t, expectedMappings, mappings)
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

func readAllCallbacks(t *testing.T, db *sql.DB) []*store.CallbackData {
	t.Helper()

	r, err := db.Query(
		`SELECT url
			,token
			,tx_id
			,tx_status
			,extra_info
			,merkle_path
			,block_hash
			,block_height
			,timestamp
			,competing_txs
		FROM callbacker.callbacks`,
	)
	require.NoError(t, err)

	defer r.Close()

	var callbacks []*store.CallbackData

	for r.Next() {
		c := &store.CallbackData{}
		var ei sql.NullString
		var mp sql.NullString
		var bh sql.NullString
		var bheight sql.NullInt64
		var competingTxs sql.NullString

		_ = r.Scan(&c.URL, &c.Token, &c.TxID, &c.TxStatus, &ei, &mp, &bh, &bheight, &c.Timestamp, &competingTxs)

		if ei.Valid {
			c.ExtraInfo = &ei.String
		}
		if mp.Valid {
			c.MerklePath = &mp.String
		}
		if bh.Valid {
			c.BlockHash = &bh.String
		}
		bheightUint64, err := safecast.ToUint64(bheight.Int64)
		require.NoError(t, err)
		if bheight.Valid {
			c.BlockHeight = ptrTo(bheightUint64)
		}
		if competingTxs.Valid {
			c.CompetingTxs = strings.Split(competingTxs.String, ",")
		}

		c.Timestamp = c.Timestamp.UTC()

		callbacks = append(callbacks, c)
	}

	return callbacks
}
