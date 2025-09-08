package postgresql

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/ccoveille/go-safecast"
	_ "github.com/golang-migrate/migrate/v4/source/file"
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

	t.Run("insert", func(t *testing.T) {
		// given
		defer pruneTables(t, postgresDB.db)
		testutils.LoadFixtures(t, postgresDB.db, "fixtures/insert")

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
			URL:         "https://arc-callback-2/callback",
			Token:       "token",
			TxID:        "96cbf8ba96dc3bad6ecc19ce34d1edbf57b2bc6f76cc3d80efdca95599cf5c28",
			TxStatus:    "MINED",
			Timestamp:   now,
			BlockHash:   &testdata.Block1,
			BlockHeight: ptrTo(uint64(4524235)),
		}
		cbData5 := &store.CallbackData{
			URL:         "https://test-callback-2/",
			Token:       "token",
			TxID:        testdata.TX2,
			TxStatus:    "MINED",
			Timestamp:   now,
			BlockHash:   ptrTo(testdata.Block1),
			BlockHeight: ptrTo(uint64(4524235)),
		}
		cbData6 := &store.CallbackData{ // pre-existing in DB
			URL:         "https://arc-callback-1/callback",
			Token:       "token",
			TxID:        "96cbf8ba96dc3bad6ecc19ce34d1edbf57b2bc6f76cc3d80efdca95599cf5c28",
			TxStatus:    "MINED",
			Timestamp:   now,
			BlockHash:   &testdata.Block1,
			BlockHeight: ptrTo(uint64(4524235)),
		}
		cbData7 := &store.CallbackData{ // differing block hash
			URL:         "https://arc-callback-1/callback",
			Token:       "token",
			TxID:        "96cbf8ba96dc3bad6ecc19ce34d1edbf57b2bc6f76cc3d80efdca95599cf5c28",
			TxStatus:    "MINED",
			Timestamp:   now,
			BlockHash:   &testdata.Block2,
			BlockHeight: ptrTo(uint64(4524235)),
		}

		data := []*store.CallbackData{
			cbData1,
			cbData2,
			cbData2, // duplicate
			cbData3,
			cbData3, // duplicate
			cbData4,
			cbData5,
			cbData7,
		}

		// when
		rows, err := postgresDB.Insert(context.Background(), data)
		// then
		require.NoError(t, err)

		require.Equal(t, rows, int64(6))

		dbCallbacks := readAllCallbacks(t, postgresDB.db)
		require.NoError(t, err)

		expected := []*store.CallbackData{
			cbData1,
			cbData2,
			cbData3,
			cbData4,
			cbData5,
			cbData6,
			cbData7,
		}

		require.ElementsMatch(t, expected, dbCallbacks)
	})

	t.Run("set sent", func(t *testing.T) {
		defer pruneTables(t, postgresDB.db)
		testutils.LoadFixtures(t, postgresDB.db, "fixtures/set_sent")
		ctx := context.Background()

		err = postgresDB.SetSent(ctx, []int64{1, 2, 3})
		require.NoError(t, err)

		r, err := postgresDB.db.QueryContext(ctx,
			`SELECT sent_at, pending FROM callbacker.transaction_callbacks WHERE id = ANY($1::INTEGER[])`,
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

	t.Run("unset pending", func(t *testing.T) {
		defer pruneTables(t, postgresDB.db)
		testutils.LoadFixtures(t, postgresDB.db, "fixtures/unset_pending")
		ctx := context.Background()

		err = postgresDB.UnsetPending(ctx, []int64{1, 2, 3})
		require.NoError(t, err)

		r, err := postgresDB.db.QueryContext(ctx,
			`SELECT pending FROM callbacker.transaction_callbacks WHERE id = ANY($1::INTEGER[])`,
			pq.Array([]int64{1, 2, 3}),
		)
		require.NoError(t, err)

		for r.Next() {
			var pending sql.NullTime
			err = r.Scan(&pending)
			require.NoError(t, err)
			require.False(t, pending.Valid)
		}
	})

	t.Run("get unsent", func(t *testing.T) {
		// given
		defer pruneTables(t, postgresDB.db)
		testutils.LoadFixtures(t, postgresDB.db, "fixtures/get_unsent")
		ctx := context.Background()

		const limit = 20
		records, err := postgresDB.GetUnsent(ctx, limit, 1*time.Hour, false)
		require.NoError(t, err)
		require.Len(t, records, 13)

		expectedTxIDs := map[string]struct{}{
			"3413cc9b40d48661c7f36bee88ebb39fca1d593f9672f840afdf07b018e73bb7": {},
			"277fb619a6ee37757123301fce61884e741ab4e01a0dea7ec465ae74f43f82cc": {},
			"13441601b8f4bd6062ce113118e957c04442a3293360fffbe0ed8805c34c6343": {},
			"862f7781e2e65efddd3cb6bb3a924ea53edba299354991eb38bf47ec6e5c986c": {},
			"0e27376ded97b656ccc02ecd6948d2e775d8904e7093eb89e8d2bf1eb7a60ea9": {},
			"6bcfe17b6b41511ee891401998caefb7ffdcee87653e197dfb1add5860b6a070": {},
			"c50eeb84c58780bb0eb39f430deb93e5f362fe16b73aa4f811c089e14a1815ae": {},
		}

		ids := make([]int64, len(records))
		for i, record := range records {
			_, found := expectedTxIDs[record.TxID]
			require.True(t, found)
			ids[i] = record.ID
		}

		r, err := postgresDB.db.QueryContext(ctx,
			`SELECT pending FROM callbacker.transaction_callbacks WHERE id = ANY($1::INTEGER[])`,
			pq.Array(ids),
		)
		require.NoError(t, err)
		defer r.Close()

		for r.Next() {
			var pending sql.NullTime
			err = r.Scan(&pending)
			require.NoError(t, err)
			require.True(t, pending.Valid)
			require.Equal(t, now.UTC(), pending.Time.UTC())
		}
	})

	t.Run("clear", func(t *testing.T) {
		// given
		defer pruneTables(t, postgresDB.db)
		testutils.LoadFixtures(t, postgresDB.db, "fixtures/clear")
		ctx := context.Background()

		var rowsBefore int
		err = postgresDB.db.QueryRowContext(ctx, "SELECT COUNT(1) FROM callbacker.transaction_callbacks").Scan(&rowsBefore)
		require.NoError(t, err)
		require.Equal(t, 30, rowsBefore)

		err := postgresDB.Clear(ctx, now)
		require.NoError(t, err)

		var rowsAfter int
		err = postgresDB.db.QueryRowContext(ctx, "SELECT COUNT(1) FROM callbacker.transaction_callbacks").Scan(&rowsAfter)
		require.NoError(t, err)

		// then
		require.Equal(t, 3, rowsAfter)
	})
}

func pruneTables(t *testing.T, db *sql.DB) {
	testutils.PruneTables(t, db, "callbacker.callbacks")
	testutils.PruneTables(t, db, "callbacker.transaction_callbacks")
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
		FROM callbacker.transaction_callbacks`,
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
