package postgresql

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"flag"
	"log"
	"os"
	"testing"
	"time"

	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
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
		os.Exit(0)
	}

	os.Exit(testmain(m))
}

func testmain(m *testing.M) int {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Printf("failed to create pool: %v", err)
		return 1
	}

	port := "5433"
	resource, connStr, err := testutils.RunAndMigratePostgresql(pool, port, "metamorph", migrationsPath)
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

func pruneTables(t *testing.T, db *sql.DB) {
	testutils.PruneTables(t, db, "metamorph.transactions")
}

func TestPostgresDB(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	now := time.Date(2023, 10, 1, 14, 25, 0, 0, time.UTC)
	minedHash := testdata.TX1Hash
	minedData := &store.Data{
		RawTx:         make([]byte, 0),
		StoredAt:      now,
		Hash:          minedHash,
		Status:        metamorph_api.Status_MINED,
		BlockHeight:   100,
		BlockHash:     testdata.Block1Hash,
		Callbacks:     []store.Callback{{CallbackURL: "http://callback.example.com", CallbackToken: "12345"}},
		RejectReason:  "not rejected",
		LockedBy:      "metamorph-1",
		StatusHistory: make([]*store.StatusWithTimestamp, 0),
	}

	unminedHash := testdata.TX1Hash
	unminedData := &store.Data{
		RawTx:    make([]byte, 0),
		StoredAt: now,
		Hash:     unminedHash,
		Status:   metamorph_api.Status_UNKNOWN,
		LockedBy: "metamorph-1",
	}

	postgresDB, err := New(dbInfo, "metamorph-1", 10, 10, WithNow(func() time.Time {
		return now
	}))
	ctx := context.Background()
	require.NoError(t, err)
	defer func() {
		postgresDB.Close(ctx)
	}()

	t.Run("get/set/del", func(t *testing.T) {
		defer pruneTables(t, postgresDB.db)

		mined := *minedData
		err = postgresDB.Set(ctx, &mined)
		require.NoError(t, err)

		dataReturned, err := postgresDB.Get(ctx, minedHash[:])
		require.NoError(t, err)
		mined.LastModified = dataReturned.LastModified
		require.Equal(t, dataReturned, &mined)

		mined.LastSubmittedAt = time.Date(2024, 5, 31, 15, 16, 0, 0, time.UTC)
		err = postgresDB.Set(ctx, &mined)
		require.NoError(t, err)

		dataReturned2, err := postgresDB.Get(ctx, minedHash[:])
		require.NoError(t, err)
		require.Equal(t, time.Date(2024, 5, 31, 15, 16, 0, 0, time.UTC), dataReturned2.LastSubmittedAt)

		mined.Callbacks = append(mined.Callbacks, store.Callback{CallbackURL: "http://callback.example2.com", CallbackToken: "67890"})
		err = postgresDB.Set(ctx, &mined)
		require.NoError(t, err)

		dataReturned3, err := postgresDB.Get(ctx, minedHash[:])
		require.NoError(t, err)
		require.Equal(t, dataReturned3.Callbacks, mined.Callbacks)

		err = postgresDB.Del(ctx, minedHash[:])
		require.NoError(t, err)

		_, err = postgresDB.Get(ctx, minedHash[:])
		require.True(t, errors.Is(err, store.ErrNotFound))

		_, err = postgresDB.Get(ctx, []byte("not to be found"))
		require.True(t, errors.Is(err, store.ErrNotFound))
	})

	t.Run("get raw txs", func(t *testing.T) {
		defer pruneTables(t, postgresDB.db)
		testutils.LoadFixtures(t, postgresDB.db, "fixtures/get_rawtxs")

		hash1 := "cd3d2f97dfc0cdb6a07ec4b72df5e1794c9553ff2f62d90ed4add047e8088853"
		hash2 := "21132d32cb5411c058bb4391f24f6a36ed9b810df851d0e36cac514fd03d6b4e"
		hash3 := "3e0b5b218c344110f09bf485bc58de4ea5378e55744185edf9c1dafa40068ecd"

		hashes := make([][]byte, 0)
		hashes = append(hashes, testutils.RevChainhash(t, hash1).CloneBytes())
		hashes = append(hashes, testutils.RevChainhash(t, hash2).CloneBytes())
		hashes = append(hashes, testutils.RevChainhash(t, hash3).CloneBytes())

		expectedRawTxs := make([][]byte, 0)

		rawTx, err := hex.DecodeString("010000000000000000ef016f8828b2d3f8085561d0b4ff6f5d17c269206fa3d32bcd3b22e26ce659ed12e7000000006b483045022100d3649d120249a09af44b4673eecec873109a3e120b9610b78858087fb225c9b9022037f16999b7a4fecdd9f47ebdc44abd74567a18940c37e1481ab0fe84d62152e4412102f87ce69f6ba5444aed49c34470041189c1e1060acd99341959c0594002c61bf0ffffffffe7030000000000001976a914c2b6fd4319122b9b5156a2a0060d19864c24f49a88ac01e7030000000000001976a914c2b6fd4319122b9b5156a2a0060d19864c24f49a88ac00000000")
		require.NoError(t, err)
		expectedRawTxs = append(expectedRawTxs, rawTx)

		rawTx, err = hex.DecodeString("020000000000000000ef016f8828b2d3f8085561d0b4ff6f5d17c269206fa3d32bcd3b22e26ce659ed12e7000000006b483045022100d3649d120249a09af44b4673eecec873109a3e120b9610b78858087fb225c9b9022037f16999b7a4fecdd9f47ebdc44abd74567a18940c37e1481ab0fe84d62152e4412102f87ce69f6ba5444aed49c34470041189c1e1060acd99341959c0594002c61bf0ffffffffe7030000000000001976a914c2b6fd4319122b9b5156a2a0060d19864c24f49a88ac01e7030000000000001976a914c2b6fd4319122b9b5156a2a0060d19864c24f49a88ac00000000")
		require.NoError(t, err)
		expectedRawTxs = append(expectedRawTxs, rawTx)

		rawTx, err = hex.DecodeString("030000000000000000ef016f8828b2d3f8085561d0b4ff6f5d17c269206fa3d32bcd3b22e26ce659ed12e7000000006b483045022100d3649d120249a09af44b4673eecec873109a3e120b9610b78858087fb225c9b9022037f16999b7a4fecdd9f47ebdc44abd74567a18940c37e1481ab0fe84d62152e4412102f87ce69f6ba5444aed49c34470041189c1e1060acd99341959c0594002c61bf0ffffffffe7030000000000001976a914c2b6fd4319122b9b5156a2a0060d19864c24f49a88ac01e7030000000000001976a914c2b6fd4319122b9b5156a2a0060d19864c24f49a88ac00000000")
		require.NoError(t, err)
		expectedRawTxs = append(expectedRawTxs, rawTx)

		rawTxs, err := postgresDB.GetRawTxs(context.Background(), hashes)
		require.NoError(t, err)

		assert.Equal(t, expectedRawTxs, rawTxs)
	})

	t.Run("get many", func(t *testing.T) {
		// when
		defer pruneTables(t, postgresDB.db)
		testutils.LoadFixtures(t, postgresDB.db, "fixtures/transactions")

		keys := [][]byte{
			testutils.RevChainhash(t, "cd3d2f97dfc0cdb6a07ec4b72df5e1794c9553ff2f62d90ed4add047e8088853")[:],
			testutils.RevChainhash(t, "ee76f5b746893d3e6ae6a14a15e464704f4ebd601537820933789740acdcf6aa")[:],
		}

		// then
		res, err := postgresDB.GetMany(ctx, keys)

		// assert
		require.NoError(t, err)
		require.Len(t, res, len(keys))
	})

	t.Run("set bulk", func(t *testing.T) {
		defer pruneTables(t, postgresDB.db)
		testutils.LoadFixtures(t, postgresDB.db, "fixtures/set_bulk")

		hash2 := testutils.RevChainhash(t, "cd3d2f97dfc0cdb6a07ec4b72df5e1794c9553ff2f62d90ed4add047e8088853") // hash already existing in db - no update expected

		data := []*store.Data{
			{
				RawTx:             testdata.TX1Raw.Bytes(),
				StoredAt:          now,
				Hash:              testdata.TX1Hash,
				Status:            metamorph_api.Status_STORED,
				Callbacks:         []store.Callback{{CallbackURL: "http://callback.example.com", CallbackToken: "1234"}},
				FullStatusUpdates: false,
				LastSubmittedAt:   now,
				LockedBy:          "metamorph-1",
			},
			{
				RawTx:             testdata.TX6Raw.Bytes(),
				StoredAt:          now,
				Hash:              testdata.TX6Hash,
				Status:            metamorph_api.Status_STORED,
				Callbacks:         []store.Callback{{CallbackURL: "http://callback.example2.com", CallbackToken: "5678"}},
				FullStatusUpdates: true,
				LastSubmittedAt:   now,
				LockedBy:          "metamorph-1",
			},
			{
				RawTx:             testdata.TX6Raw.Bytes(),
				StoredAt:          now,
				Hash:              hash2,
				Status:            metamorph_api.Status_STORED,
				Callbacks:         []store.Callback{{CallbackURL: "http://callback.example3.com", CallbackToken: "5678"}},
				FullStatusUpdates: true,
				LastSubmittedAt:   now,
				LockedBy:          "metamorph-1",
			},
		}

		err = postgresDB.SetBulk(ctx, data)
		require.NoError(t, err)

		data0, err := postgresDB.Get(ctx, testdata.TX1Hash[:])
		require.NoError(t, err)
		data[0].LastModified = data0.LastModified
		require.Equal(t, data[0], data0)

		data1, err := postgresDB.Get(ctx, testdata.TX6Hash[:])
		require.NoError(t, err)
		data[1].LastModified = data1.LastModified
		require.Equal(t, data[1], data1)

		data2, err := postgresDB.Get(ctx, hash2[:])
		require.NoError(t, err)
		require.NotEqual(t, data[2], data2)
		require.Equal(t, metamorph_api.Status_SENT_TO_NETWORK, data2.Status)
		require.Equal(t, "metamorph-3", data2.LockedBy)
		require.Equal(t, now, data2.LastSubmittedAt)
	})

	t.Run("get unmined", func(t *testing.T) {
		defer pruneTables(t, postgresDB.db)
		testutils.LoadFixtures(t, postgresDB.db, "fixtures/transactions")

		// locked by metamorph-1
		expectedHash0 := testutils.RevChainhash(t, "3e0b5b218c344110f09bf485bc58de4ea5378e55744185edf9c1dafa40068ecd")
		// offset 0
		records, err := postgresDB.GetUnseen(ctx, time.Date(2023, 1, 1, 1, 0, 0, 0, time.UTC), 1, 0)
		require.NoError(t, err)
		require.Equal(t, expectedHash0, records[0].Hash)

		// locked by metamorph-1
		expectedHash1 := testutils.RevChainhash(t, "b16cea53fc823e146fbb9ae4ad3124f7c273f30562585ad6e4831495d609f430")
		// offset 1
		records, err = postgresDB.GetUnseen(ctx, time.Date(2023, 1, 1, 1, 0, 0, 0, time.UTC), 1, 1)
		require.NoError(t, err)
		require.Equal(t, expectedHash1, records[0].Hash)
	})

	t.Run("set locked by", func(t *testing.T) {
		defer pruneTables(t, postgresDB.db)
		testutils.LoadFixtures(t, postgresDB.db, "fixtures/transactions")

		err := postgresDB.SetLocked(ctx, time.Date(2023, 9, 15, 1, 0, 0, 0, time.UTC), 2)
		require.NoError(t, err)

		// locked by NONE
		expectedHash2 := testutils.RevChainhash(t, "12c04cfc5643f1cd25639ad42d6f8f0489557699d92071d7e0a5b940438c4357")

		// locked by NONE
		expectedHash3 := testutils.RevChainhash(t, "319b5eb9d99084b72002640d1445f49b8c83539260a7e5b2cbb16c1d2954a743")

		// check if previously unlocked tx has b
		// locked by NONE
		expectedHash4 := testutils.RevChainhash(t, "78d66c8391ff5e4a65b494e39645facb420b744f77f3f3b83a3aa8573282176e")

		// check if previously unlocked tx has been locked
		dataReturned, err := postgresDB.Get(ctx, expectedHash2[:])
		require.NoError(t, err)
		require.Equal(t, "metamorph-1", dataReturned.LockedBy)

		dataReturned, err = postgresDB.Get(ctx, expectedHash3[:])
		require.NoError(t, err)
		require.Equal(t, "metamorph-1", dataReturned.LockedBy)

		// this unlocked tx remains unlocked as the limit was 2
		dataReturned, err = postgresDB.Get(ctx, expectedHash4[:])
		require.NoError(t, err)
		require.Equal(t, "NONE", dataReturned.LockedBy)
	})

	t.Run("set unlocked by name", func(t *testing.T) {
		defer pruneTables(t, postgresDB.db)
		testutils.LoadFixtures(t, postgresDB.db, "fixtures/set_unlocked_by_name")

		rows, err := postgresDB.SetUnlockedByName(ctx, "metamorph-3")
		require.NoError(t, err)
		require.Equal(t, int64(4), rows)

		hash1 := testutils.RevChainhash(t, "cd3d2f97dfc0cdb6a07ec4b72df5e1794c9553ff2f62d90ed4add047e8088853")
		hash1Data, err := postgresDB.Get(ctx, hash1[:])
		require.NoError(t, err)
		require.Equal(t, "NONE", hash1Data.LockedBy)

		hash2 := testutils.RevChainhash(t, "21132d32cb5411c058bb4391f24f6a36ed9b810df851d0e36cac514fd03d6b4e")
		hash2Data, err := postgresDB.Get(ctx, hash2[:])
		require.NoError(t, err)
		require.Equal(t, "NONE", hash2Data.LockedBy)

		hash3 := testutils.RevChainhash(t, "f791ec50447e3001b9348930659527ea92dee506e9950014bcc7c5b146e2417f")
		hash3Data, err := postgresDB.Get(ctx, hash3[:])
		require.NoError(t, err)
		require.Equal(t, "NONE", hash3Data.LockedBy)

		hash4 := testutils.RevChainhash(t, "89714f129748e5176a07fc4eb89cf27a9e60340117e6b56bb742acb2873f8140")
		hash4Data, err := postgresDB.Get(ctx, hash4[:])
		require.NoError(t, err)
		require.Equal(t, "NONE", hash4Data.LockedBy)
	})

	t.Run("set unlocked by name except", func(t *testing.T) {
		defer pruneTables(t, postgresDB.db)
		testutils.LoadFixtures(t, postgresDB.db, "fixtures/set_unlocked_by_name")

		rows, err := postgresDB.SetUnlockedByNameExcept(ctx, []string{"metamorph-1", ""})
		require.NoError(t, err)
		require.Equal(t, int64(4), rows)

		hash1 := testutils.RevChainhash(t, "cd3d2f97dfc0cdb6a07ec4b72df5e1794c9553ff2f62d90ed4add047e8088853")
		hash1Data, err := postgresDB.Get(ctx, hash1[:])
		require.NoError(t, err)
		require.Equal(t, "NONE", hash1Data.LockedBy)

		hash2 := testutils.RevChainhash(t, "21132d32cb5411c058bb4391f24f6a36ed9b810df851d0e36cac514fd03d6b4e")
		hash2Data, err := postgresDB.Get(ctx, hash2[:])
		require.NoError(t, err)
		require.Equal(t, "NONE", hash2Data.LockedBy)

		hash3 := testutils.RevChainhash(t, "f791ec50447e3001b9348930659527ea92dee506e9950014bcc7c5b146e2417f")
		hash3Data, err := postgresDB.Get(ctx, hash3[:])
		require.NoError(t, err)
		require.Equal(t, "NONE", hash3Data.LockedBy)

		hash4 := testutils.RevChainhash(t, "89714f129748e5176a07fc4eb89cf27a9e60340117e6b56bb742acb2873f8140")
		hash4Data, err := postgresDB.Get(ctx, hash4[:])
		require.NoError(t, err)
		require.Equal(t, "NONE", hash4Data.LockedBy)
	})

	t.Run("update status", func(t *testing.T) {
		defer pruneTables(t, postgresDB.db)
		testutils.LoadFixtures(t, postgresDB.db, "fixtures/update_status")

		updates := []store.UpdateStatus{
			{
				Hash:   *testutils.RevChainhash(t, "cd3d2f97dfc0cdb6a07ec4b72df5e1794c9553ff2f62d90ed4add047e8088853"), // update expected
				Status: metamorph_api.Status_ACCEPTED_BY_NETWORK,
			},
			{
				Hash:   *testutils.RevChainhash(t, "21132d32cb5411c058bb4391f24f6a36ed9b810df851d0e36cac514fd03d6b4e"), // update not expected - old status = new status
				Status: metamorph_api.Status_REQUESTED_BY_NETWORK,
			},
			{
				Hash:   *testutils.RevChainhash(t, "b16cea53fc823e146fbb9ae4ad3124f7c273f30562585ad6e4831495d609f430"), // update expected
				Status: metamorph_api.Status_REJECTED,
				Error:  errors.New("missing inputs"),
			},
			{
				Hash:   *testutils.RevChainhash(t, "ee76f5b746893d3e6ae6a14a15e464704f4ebd601537820933789740acdcf6aa"), // update expected
				Status: metamorph_api.Status_SEEN_ON_NETWORK,
			},
			{
				Hash:   *testutils.RevChainhash(t, "3e0b5b218c344110f09bf485bc58de4ea5378e55744185edf9c1dafa40068ecd"), // update not expected - status is mined
				Status: metamorph_api.Status_SENT_TO_NETWORK,
			},
			{
				Hash:   *testutils.RevChainhash(t, "7809b730cbe7bb723f299a4e481fb5165f31175876392a54cde85569a18cc75f"), // update not expected - old status > new status
				Status: metamorph_api.Status_SENT_TO_NETWORK,
			},
			{
				Hash:   *testutils.RevChainhash(t, "3ce1e0c6cbbbe2118c3f80d2e6899d2d487f319ef0923feb61f3d26335b2225c"), // update not expected - hash non-existent in db
				Status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
			},
			{
				Hash:   *testutils.RevChainhash(t, "7e3350ca12a0dd9375540e13637b02e054a3436336e9d6b82fe7f2b23c710002"), // update not expected - hash non-existent in db
				Status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
			},
		}
		updatedStatuses := 3

		statusUpdates, err := postgresDB.UpdateStatus(ctx, updates)
		require.NoError(t, err)
		require.Len(t, statusUpdates, updatedStatuses)

		require.Equal(t, metamorph_api.Status_ACCEPTED_BY_NETWORK, statusUpdates[0].Status)
		require.Equal(t, *testutils.RevChainhash(t, "cd3d2f97dfc0cdb6a07ec4b72df5e1794c9553ff2f62d90ed4add047e8088853"), *statusUpdates[0].Hash)

		require.Equal(t, metamorph_api.Status_REJECTED, statusUpdates[1].Status)
		require.Equal(t, "missing inputs", statusUpdates[1].RejectReason)
		require.Equal(t, *testutils.RevChainhash(t, "b16cea53fc823e146fbb9ae4ad3124f7c273f30562585ad6e4831495d609f430"), *statusUpdates[1].Hash)

		require.Equal(t, metamorph_api.Status_SEEN_ON_NETWORK, statusUpdates[2].Status)
		require.Equal(t, *testutils.RevChainhash(t, "ee76f5b746893d3e6ae6a14a15e464704f4ebd601537820933789740acdcf6aa"), *statusUpdates[2].Hash)

		returnedDataRequested, err := postgresDB.Get(ctx, testutils.RevChainhash(t, "7809b730cbe7bb723f299a4e481fb5165f31175876392a54cde85569a18cc75f")[:])
		require.NoError(t, err)
		require.Equal(t, metamorph_api.Status_ACCEPTED_BY_NETWORK, returnedDataRequested.Status)

		statusUpdates, err = postgresDB.UpdateStatus(ctx, updates)
		require.NoError(t, err)
		require.Len(t, statusUpdates, 0)
	})

	t.Run("update double spend status", func(t *testing.T) {
		defer pruneTables(t, postgresDB.db)
		testutils.LoadFixtures(t, postgresDB.db, "fixtures/update_double_spend")

		timestamp := time.Date(2025, 4, 3, 14, 5, 20, 0, time.UTC)

		updates := []store.UpdateStatus{
			{
				Hash:         *testutils.RevChainhash(t, "cd3d2f97dfc0cdb6a07ec4b72df5e1794c9553ff2f62d90ed4add047e8088853"), // update expected
				Status:       metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED,
				CompetingTxs: []string{"5555532d32cb5411c058bb4391f24f6a36ed9b810df851d0e36cac514fd03d6b4e"},
				Timestamp:    timestamp,
			},
			{
				Hash:         *testutils.RevChainhash(t, "21132d32cb5411c058bb4391f24f6a36ed9b810df851d0e36cac514fd03d6b4e"), // update expected
				Status:       metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED,
				CompetingTxs: []string{"11132d32cb5411c058bb4391f24f6a36ed9b810df851d0e36cac514fd03d6b4e", "22232d32cb5411c058bb4391f24f6a36ed9b810df851d0e36cac514fd03d6b4e"},
				Timestamp:    timestamp,
			},
			{
				Hash:         *testutils.RevChainhash(t, "b16cea53fc823e146fbb9ae4ad3124f7c273f30562585ad6e4831495d609f430"), // update expected
				Status:       metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED,
				CompetingTxs: []string{"33332d32cb5411c058bb4391f24f6a36ed9b810df851d0e36cac514fd03d6b4e"},
				Timestamp:    timestamp,
			},
			{
				Hash:         *testutils.RevChainhash(t, "3e0b5b218c344110f09bf485bc58de4ea5378e55744185edf9c1dafa40068ecd"), // update expected
				Status:       metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED,
				CompetingTxs: []string{"aaa350ca12a0dd9375540e13637b02e054a3436336e9d6b82fe7f2b23c710002"},
				Timestamp:    timestamp,
			},
			{
				Hash:         *testutils.RevChainhash(t, "7809b730cbe7bb723f299a4e481fb5165f31175876392a54cde85569a18cc75f"), // update expected - old status < new status
				Status:       metamorph_api.Status_REJECTED,
				CompetingTxs: []string{"33332d32cb5411c058bb4391f24f6a36ed9b810df851d0e36cac514fd03d6b4e"},
				Error:        errors.New("double spend attempted"),
				Timestamp:    timestamp,
			},
			{
				Hash:         *testutils.RevChainhash(t, "3ce1e0c6cbbbe2118c3f80d2e6899d2d487f319ef0923feb61f3d26335b2225c"), // update not expected - hash non-existent in db
				Status:       metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED,
				CompetingTxs: []string{"33332d32cb5411c058bb4391f24f6a36ed9b810df851d0e36cac514fd03d6b4e"},
				Timestamp:    timestamp,
			},
			{
				Hash:         *testutils.RevChainhash(t, "7e3350ca12a0dd9375540e13637b02e054a3436336e9d6b82fe7f2b23c710002"), // update not expected - hash non-existent in db
				Status:       metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED,
				CompetingTxs: []string{"33332d32cb5411c058bb4391f24f6a36ed9b810df851d0e36cac514fd03d6b4e"},
				Timestamp:    timestamp,
			},
		}
		updatedStatuses := 5
		statusUpdates, err := postgresDB.UpdateDoubleSpend(ctx, updates, true)
		require.NoError(t, err)
		require.Len(t, statusUpdates, updatedStatuses)

		require.Equal(t, metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED, statusUpdates[0].Status)
		require.Equal(t, *testutils.RevChainhash(t, "cd3d2f97dfc0cdb6a07ec4b72df5e1794c9553ff2f62d90ed4add047e8088853"), *statusUpdates[0].Hash)
		require.ElementsMatch(t, []*store.StatusWithTimestamp{
			{Status: metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED, Timestamp: timestamp},
			{Status: metamorph_api.Status_ANNOUNCED_TO_NETWORK, Timestamp: time.Date(2023, 10, 1, 14, 0, 0, 0, time.UTC)},
		}, statusUpdates[0].StatusHistory)
		require.ElementsMatch(t, []string{"5555532d32cb5411c058bb4391f24f6a36ed9b810df851d0e36cac514fd03d6b4e", "33332d32cb5411c058bb4391f24f6a36ed9b810df851d0e36cac514fd03d6b4e"}, statusUpdates[0].CompetingTxs)
		require.Equal(t, metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED, statusUpdates[1].Status)
		require.Equal(t, *testutils.RevChainhash(t, "21132d32cb5411c058bb4391f24f6a36ed9b810df851d0e36cac514fd03d6b4e"), *statusUpdates[1].Hash)
		require.Equal(t, []*store.StatusWithTimestamp{{Status: metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED, Timestamp: timestamp}}, statusUpdates[1].StatusHistory)
		require.ElementsMatch(t, []string{"11132d32cb5411c058bb4391f24f6a36ed9b810df851d0e36cac514fd03d6b4e", "22232d32cb5411c058bb4391f24f6a36ed9b810df851d0e36cac514fd03d6b4e", "33332d32cb5411c058bb4391f24f6a36ed9b810df851d0e36cac514fd03d6b4e", "5555532d32cb5411c058bb4391f24f6a36ed9b810df851d0e36cac514fd03d6b4e"}, statusUpdates[1].CompetingTxs)

		require.Equal(t, metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED, statusUpdates[2].Status)
		require.Equal(t, *testutils.RevChainhash(t, "b16cea53fc823e146fbb9ae4ad3124f7c273f30562585ad6e4831495d609f430"), *statusUpdates[2].Hash)
		require.Equal(t, []*store.StatusWithTimestamp{{Status: metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED, Timestamp: timestamp}}, statusUpdates[2].StatusHistory)
		require.Equal(t, []string{"33332d32cb5411c058bb4391f24f6a36ed9b810df851d0e36cac514fd03d6b4e"}, statusUpdates[2].CompetingTxs)
		require.Equal(t, metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED, statusUpdates[3].Status)
		require.Equal(t, *testutils.RevChainhash(t, "3e0b5b218c344110f09bf485bc58de4ea5378e55744185edf9c1dafa40068ecd"), *statusUpdates[3].Hash)
		require.Equal(t, []string{"aaa350ca12a0dd9375540e13637b02e054a3436336e9d6b82fe7f2b23c710002"}, statusUpdates[3].CompetingTxs)
		require.Equal(t, metamorph_api.Status_REJECTED, statusUpdates[4].Status)
		require.Equal(t, *testutils.RevChainhash(t, "7809b730cbe7bb723f299a4e481fb5165f31175876392a54cde85569a18cc75f"), *statusUpdates[4].Hash)
		require.Equal(t, []*store.StatusWithTimestamp{{Status: metamorph_api.Status_REJECTED, Timestamp: timestamp}}, statusUpdates[4].StatusHistory)
		require.Equal(t, []string{"33332d32cb5411c058bb4391f24f6a36ed9b810df851d0e36cac514fd03d6b4e"}, statusUpdates[4].CompetingTxs)
		require.Equal(t, "double spend attempted", statusUpdates[4].RejectReason)
		res, err := postgresDB.Get(ctx, testutils.RevChainhash(t, "aaa350ca12a0dd9375540e13637b02e054a3436336e9d6b82fe7f2b23c710002")[:])
		require.NoError(t, err)
		require.Equal(t, metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED, res.Status)
		statusUpdates, err = postgresDB.UpdateDoubleSpend(ctx, updates, false)
		require.NoError(t, err)
		require.Len(t, statusUpdates, 0)
	})

	t.Run("update mined", func(t *testing.T) {
		defer pruneTables(t, postgresDB.db)
		testutils.LoadFixtures(t, postgresDB.db, "fixtures/transactions")

		unmined := *unminedData
		err = postgresDB.Set(ctx, &unmined)
		require.NoError(t, err)

		chainHash2 := testutils.RevChainhash(t, "ee76f5b746893d3e6ae6a14a15e464704f4ebd601537820933789740acdcf6aa")
		chainHash3 := testutils.RevChainhash(t, "a7fd98bd37f9b387dbef4f1a4e4790b9a0d48fb7bbb77455e8f39df0f8909db7")
		competingHash := testutils.RevChainhash(t, "67fc757d9ed6d119fc0926ae5c82c1a2cf036ec823257cfaea396e49184ec7ff")
		chainhash4 := testutils.RevChainhash(t, "3e0b5b218c344110f09bf485bc58de4ea5378e55744185edf9c1dafa40068ecd")

		txBlocks := []*blocktx_api.TransactionBlock{
			{
				BlockHash:       testdata.Block1Hash[:],
				BlockHeight:     100,
				TransactionHash: unminedHash[:],
				MerklePath:      "merkle-path-1",
				BlockStatus:     blocktx_api.Status_LONGEST,
			},
			{
				BlockHash:       testdata.Block1Hash[:],
				BlockHeight:     100,
				TransactionHash: chainHash2[:],
				MerklePath:      "merkle-path-2",
				BlockStatus:     blocktx_api.Status_LONGEST,
			},
			{
				BlockHash:       testdata.Block1Hash[:],
				BlockHeight:     100,
				TransactionHash: testdata.TX3Hash[:], // hash non-existent in db
				MerklePath:      "merkle-path-3",
				BlockStatus:     blocktx_api.Status_LONGEST,
			},
			{
				BlockHash:       testdata.Block1Hash[:],
				BlockHeight:     100,
				TransactionHash: chainHash3[:], // this one has competing transactions
				MerklePath:      "merkle-path-4",
				BlockStatus:     blocktx_api.Status_LONGEST,
			},
			{
				BlockHash:       testdata.Block2Hash[:],
				BlockHeight:     100,
				TransactionHash: chainhash4[:],
				MerklePath:      "merkle-path-5",
				BlockStatus:     blocktx_api.Status_STALE, // should have status MINED_IN_STALE_BLOCK
			},
		}
		expectedUpdates := 5 // 4 for updates + 1 for rejected competing tx

		updated, err := postgresDB.UpdateMined(ctx, txBlocks)
		require.NoError(t, err)
		require.Len(t, updated, expectedUpdates)

		require.True(t, chainHash2.IsEqual(updated[0].Hash))
		require.True(t, testdata.Block1Hash.IsEqual(updated[0].BlockHash))
		require.Equal(t, "merkle-path-2", updated[0].MerklePath)
		require.Equal(t, metamorph_api.Status_MINED, updated[0].Status)

		require.True(t, chainHash3.IsEqual(updated[1].Hash))
		require.True(t, testdata.Block1Hash.IsEqual(updated[1].BlockHash))
		require.Equal(t, "merkle-path-4", updated[1].MerklePath)
		require.Equal(t, metamorph_api.Status_MINED, updated[1].Status)

		require.True(t, chainhash4.IsEqual(updated[2].Hash))
		require.True(t, testdata.Block2Hash.IsEqual(updated[2].BlockHash))
		require.Equal(t, "merkle-path-5", updated[2].MerklePath)
		require.Equal(t, metamorph_api.Status_MINED_IN_STALE_BLOCK, updated[2].Status)

		require.True(t, unminedHash.IsEqual(updated[3].Hash))
		require.True(t, testdata.Block1Hash.IsEqual(updated[3].BlockHash))
		require.Equal(t, "merkle-path-1", updated[3].MerklePath)
		require.Equal(t, metamorph_api.Status_MINED, updated[3].Status)

		require.True(t, competingHash.IsEqual(updated[4].Hash))
		require.Equal(t, metamorph_api.Status_REJECTED, updated[4].Status)
		require.Equal(t, "double spend attempted", updated[4].RejectReason)

		minedReturned, err := postgresDB.Get(ctx, unminedHash[:])
		require.NoError(t, err)
		minedReturned.Status = metamorph_api.Status_MINED
		minedReturned.BlockHeight = 100
		minedReturned.BlockHash = testdata.Block1Hash
		minedReturned.MerklePath = "merkle-path-1"

		rejectedReturned, err := postgresDB.Get(ctx, competingHash[:])
		require.NoError(t, err)
		rejectedReturned.Status = metamorph_api.Status_REJECTED

		updated, err = postgresDB.UpdateMined(ctx, []*blocktx_api.TransactionBlock{})
		require.NoError(t, err)
		require.Len(t, updated, 0)
		require.Len(t, updated, 0)

		updated, err = postgresDB.UpdateMined(ctx, nil)
		require.NoError(t, err)
		require.Nil(t, updated)
	})

	t.Run("update mined - missing block info", func(t *testing.T) {
		defer pruneTables(t, postgresDB.db)

		unmined := *unminedData
		err = postgresDB.Set(ctx, &unmined)
		require.NoError(t, err)
		txBlocks := []*blocktx_api.TransactionBlock{{
			BlockHash:       nil,
			BlockHeight:     0,
			TransactionHash: unminedHash[:],
			MerklePath:      "",
		}}

		dataBeforeUpdate, err := postgresDB.Get(ctx, unminedHash[:])
		require.NoError(t, err)

		_, err = postgresDB.UpdateMined(ctx, txBlocks)
		require.NoError(t, err)

		dataReturned, err := postgresDB.Get(ctx, unminedHash[:])
		require.NoError(t, err)
		unmined.BlockHeight = 0
		unmined.BlockHash = nil
		unmined.StatusHistory = append(unmined.StatusHistory, &store.StatusWithTimestamp{
			Status:    metamorph_api.Status_MINED,
			Timestamp: dataBeforeUpdate.LastModified,
		})
		unmined.Status = metamorph_api.Status_MINED
		unmined.LastModified = postgresDB.now()

		require.Equal(t, &unmined, dataReturned)
	})

	t.Run("update mined - all possible updates", func(t *testing.T) {
		defer pruneTables(t, postgresDB.db)

		unmined := *unminedData
		err = postgresDB.Set(ctx, &unmined)
		require.NoError(t, err)

		// First update - UpdateStatusBulk
		// Update tx with status UNKNOWN to RECEIVED but with status history which contain QUEUED status
		updates := []store.UpdateStatus{
			{
				Hash:   *unminedHash,
				Status: metamorph_api.Status_RECEIVED,
				StatusHistory: []store.StatusWithTimestamp{
					{
						Status:    metamorph_api.Status_QUEUED,
						Timestamp: postgresDB.now(),
					},
				},
			},
		}

		statusUpdates, err := postgresDB.UpdateStatus(ctx, updates)
		require.NoError(t, err)
		require.Len(t, statusUpdates, 1)

		unmined.BlockHeight = 0
		unmined.BlockHash = nil
		unmined.StatusHistory = append(unmined.StatusHistory,
			&store.StatusWithTimestamp{
				Status:    metamorph_api.Status_QUEUED,
				Timestamp: postgresDB.now(),
			},
			&store.StatusWithTimestamp{
				Status:    metamorph_api.Status_RECEIVED,
				Timestamp: postgresDB.now(),
			},
		)
		unmined.Status = metamorph_api.Status_RECEIVED
		unmined.LastModified = postgresDB.now()

		require.Equal(t, &unmined, statusUpdates[0])

		// Update tx with status RECEIVED to ACCEPTED_BY_NETWORK without additional history
		updates = []store.UpdateStatus{
			{
				Hash:   *unminedHash,
				Status: metamorph_api.Status_ACCEPTED_BY_NETWORK,
			},
		}

		statusUpdates, err = postgresDB.UpdateStatus(ctx, updates)
		require.NoError(t, err)
		require.Len(t, statusUpdates, 1)

		unmined.StatusHistory = append(unmined.StatusHistory, &store.StatusWithTimestamp{
			Status:    metamorph_api.Status_ACCEPTED_BY_NETWORK,
			Timestamp: postgresDB.now(),
		})
		unmined.Status = metamorph_api.Status_ACCEPTED_BY_NETWORK
		unmined.LastModified = postgresDB.now()

		require.Equal(t, &unmined, statusUpdates[0])

		// Second update - UpdateDoubleSpend
		updates = []store.UpdateStatus{
			{
				Hash:         *unminedHash,
				Status:       metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED,
				CompetingTxs: []string{"5678"},
			},
		}

		statusUpdates, err = postgresDB.UpdateDoubleSpend(ctx, updates, false)
		require.NoError(t, err)
		require.Len(t, statusUpdates, 1)

		unmined.CompetingTxs = []string{"5678"}
		unmined.StatusHistory = append(unmined.StatusHistory, &store.StatusWithTimestamp{
			Status:    metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED,
			Timestamp: unmined.LastModified,
		})
		unmined.LastModified = postgresDB.now()
		unmined.Status = metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED
		require.Equal(t, &unmined, statusUpdates[0])

		// Third update - UpdateMined
		txBlocks := []*blocktx_api.TransactionBlock{
			{
				BlockHash:       testdata.Block1Hash[:],
				BlockHeight:     100,
				TransactionHash: unminedHash[:],
				MerklePath:      "merkle-path-1",
				BlockStatus:     blocktx_api.Status_LONGEST,
			},
		}

		updated, err := postgresDB.UpdateMined(ctx, txBlocks)
		require.NoError(t, err)
		require.Len(t, updated, 1)

		updatedTx, err := postgresDB.Get(ctx, unminedHash[:])
		require.NoError(t, err)

		unmined.BlockHeight = 100
		unmined.BlockHash = testdata.Block1Hash
		unmined.MerklePath = "merkle-path-1"
		unmined.StatusHistory = append(unmined.StatusHistory, &store.StatusWithTimestamp{
			Status:    metamorph_api.Status_MINED,
			Timestamp: unmined.LastModified,
		})
		unmined.LastModified = postgresDB.now()
		unmined.Status = metamorph_api.Status_MINED
		require.Equal(t, &unmined, updatedTx)

		// Fourth update - UpdateMined - MINED_IN_STALE_BLOCK
		txBlocks = []*blocktx_api.TransactionBlock{
			{
				BlockHash:       testdata.Block2Hash[:],
				BlockHeight:     100,
				TransactionHash: unminedHash[:],
				MerklePath:      "merkle-path-1",
				BlockStatus:     blocktx_api.Status_STALE,
			},
		}

		updated, err = postgresDB.UpdateMined(ctx, txBlocks)
		require.NoError(t, err)
		require.Len(t, updated, 1)

		updatedTx, err = postgresDB.Get(ctx, unminedHash[:])
		require.NoError(t, err)

		unmined.BlockHeight = 100
		unmined.BlockHash = testdata.Block2Hash
		unmined.MerklePath = "merkle-path-1"
		unmined.StatusHistory = append(unmined.StatusHistory, &store.StatusWithTimestamp{
			Status:    metamorph_api.Status_MINED_IN_STALE_BLOCK,
			Timestamp: unmined.LastModified,
		})
		unmined.LastModified = postgresDB.now()
		unmined.Status = metamorph_api.Status_MINED_IN_STALE_BLOCK
		require.Equal(t, &unmined, updatedTx)
	})

	t.Run("updated mined - reject double spending", func(t *testing.T) {
		defer pruneTables(t, postgresDB.db)
		testutils.LoadFixtures(t, postgresDB.db, "fixtures/update_mined_double_spend_attempted")

		// Even though it shouldn't happen in theory, competing transactions which have been marked of competing with each other could all be mined if the ZMQ message marking them as competing was false. Then it has to be ensured that those competing transactions should be updated as MINED only without a REJECTED update being produced

		chainHash1 := testutils.RevChainhash(t, "cd3d2f97dfc0cdb6a07ec4b72df5e1794c9553ff2f62d90ed4add047e8088853")
		chainHash2 := testutils.RevChainhash(t, "21132d32cb5411c058bb4391f24f6a36ed9b810df851d0e36cac514fd03d6b4e")
		chainHash3 := testutils.RevChainhash(t, "b16cea53fc823e146fbb9ae4ad3124f7c273f30562585ad6e4831495d609f430")

		txBlocks := []*blocktx_api.TransactionBlock{
			{
				BlockHash:       testdata.Block1Hash[:],
				BlockHeight:     100,
				TransactionHash: chainHash1[:],
				MerklePath:      "merkle-path-1",
				BlockStatus:     blocktx_api.Status_LONGEST,
			},
			{
				BlockHash:       testdata.Block1Hash[:],
				BlockHeight:     100,
				TransactionHash: chainHash2[:],
				MerklePath:      "merkle-path-2",
				BlockStatus:     blocktx_api.Status_LONGEST,
			},
			{
				BlockHash:       testdata.Block1Hash[:],
				BlockHeight:     100,
				TransactionHash: chainHash3[:],
				MerklePath:      "merkle-path-3",
				BlockStatus:     blocktx_api.Status_LONGEST,
			},
		}
		expectedUpdates := 3 // 3 for updates for mined status, no updates for rejected status

		updated, err := postgresDB.UpdateMined(ctx, txBlocks)
		require.NoError(t, err)
		require.Len(t, updated, expectedUpdates)
	})

	t.Run("update status history - status bigger than actual with history", func(t *testing.T) {
		defer pruneTables(t, postgresDB.db)

		unmined := *unminedData
		unmined.Status = metamorph_api.Status_SEEN_ON_NETWORK
		err = postgresDB.Set(ctx, &unmined)
		require.NoError(t, err)

		updates := []store.UpdateStatus{
			{
				Hash:   *unminedHash,
				Status: metamorph_api.Status_MINED,
				StatusHistory: []store.StatusWithTimestamp{
					{
						Status:    metamorph_api.Status_STORED,
						Timestamp: postgresDB.now(),
					},
				},
			},
		}

		statusUpdates, err := postgresDB.UpdateStatusHistory(ctx, updates)
		require.NoError(t, err)
		require.Len(t, statusUpdates, 1)

		unmined.StatusHistory = append(unmined.StatusHistory, &store.StatusWithTimestamp{
			Status:    metamorph_api.Status_STORED,
			Timestamp: postgresDB.now(),
		})
		unmined.LastModified = postgresDB.now()
		require.Equal(t, &unmined, statusUpdates[0])
	})

	t.Run("update status history - status equal than actual with history", func(t *testing.T) {
		defer pruneTables(t, postgresDB.db)

		unmined := *unminedData
		unmined.Status = metamorph_api.Status_SEEN_ON_NETWORK
		err = postgresDB.Set(ctx, &unmined)
		require.NoError(t, err)

		updates := []store.UpdateStatus{
			{
				Hash:   *unminedHash,
				Status: metamorph_api.Status_SEEN_ON_NETWORK,
				StatusHistory: []store.StatusWithTimestamp{
					{
						Status:    metamorph_api.Status_STORED,
						Timestamp: postgresDB.now(),
					},
					{
						Status:    metamorph_api.Status_ANNOUNCED_TO_NETWORK,
						Timestamp: postgresDB.now(),
					},
				},
			},
		}

		statusUpdates, err := postgresDB.UpdateStatusHistory(ctx, updates)
		require.NoError(t, err)
		require.Len(t, statusUpdates, 1)

		unmined.StatusHistory = append(unmined.StatusHistory, &store.StatusWithTimestamp{
			Status:    metamorph_api.Status_STORED,
			Timestamp: postgresDB.now(),
		}, &store.StatusWithTimestamp{
			Status:    metamorph_api.Status_ANNOUNCED_TO_NETWORK,
			Timestamp: postgresDB.now(),
		})
		unmined.LastModified = postgresDB.now()
		require.Equal(t, &unmined, statusUpdates[0])
	})

	t.Run("update status history - status lower than actual with history", func(t *testing.T) {
		defer pruneTables(t, postgresDB.db)

		unmined := *unminedData
		unmined.Status = metamorph_api.Status_SEEN_ON_NETWORK
		err = postgresDB.Set(ctx, &unmined)
		require.NoError(t, err)

		updates := []store.UpdateStatus{
			{
				Hash:   *unminedHash,
				Status: metamorph_api.Status_ACCEPTED_BY_NETWORK,
				StatusHistory: []store.StatusWithTimestamp{
					{
						Status:    metamorph_api.Status_STORED,
						Timestamp: postgresDB.now(),
					},
					{
						Status:    metamorph_api.Status_ANNOUNCED_TO_NETWORK,
						Timestamp: postgresDB.now(),
					},
				},
			},
		}

		statusUpdates, err := postgresDB.UpdateStatusHistory(ctx, updates)
		require.NoError(t, err)
		require.Len(t, statusUpdates, 1)

		unmined.StatusHistory = append(unmined.StatusHistory, &store.StatusWithTimestamp{
			Status:    metamorph_api.Status_STORED,
			Timestamp: postgresDB.now(),
		}, &store.StatusWithTimestamp{
			Status:    metamorph_api.Status_ANNOUNCED_TO_NETWORK,
			Timestamp: postgresDB.now(),
		}, &store.StatusWithTimestamp{
			Status:    metamorph_api.Status_ACCEPTED_BY_NETWORK,
			Timestamp: postgresDB.now(),
		})
		unmined.LastModified = postgresDB.now()
		require.Equal(t, &unmined, statusUpdates[0])
	})

	t.Run("update status history - status lower than actual without history", func(t *testing.T) {
		defer pruneTables(t, postgresDB.db)

		unmined := *unminedData
		unmined.Status = metamorph_api.Status_SEEN_ON_NETWORK
		err = postgresDB.Set(ctx, &unmined)
		require.NoError(t, err)

		updates := []store.UpdateStatus{
			{
				Hash:   *unminedHash,
				Status: metamorph_api.Status_ACCEPTED_BY_NETWORK,
			},
		}

		statusUpdates, err := postgresDB.UpdateStatusHistory(ctx, updates)
		require.NoError(t, err)
		require.Len(t, statusUpdates, 1)

		unmined.StatusHistory = append(unmined.StatusHistory, &store.StatusWithTimestamp{
			Status:    metamorph_api.Status_ACCEPTED_BY_NETWORK,
			Timestamp: postgresDB.now(),
		})
		unmined.LastModified = postgresDB.now()
		require.Equal(t, &unmined, statusUpdates[0])
	})

	t.Run("clear data", func(t *testing.T) {
		defer pruneTables(t, postgresDB.db)
		testutils.LoadFixtures(t, postgresDB.db, "fixtures/transactions")

		res, err := postgresDB.ClearData(ctx, 14)
		require.NoError(t, err)
		require.Equal(t, int64(5), res)

		var numberOfRemainingTxs int
		err = postgresDB.db.QueryRowContext(ctx, "SELECT count(*) FROM metamorph.transactions;").Scan(&numberOfRemainingTxs)
		require.NoError(t, err)
		require.Equal(t, 13, numberOfRemainingTxs)
	})

	t.Run("get seen", func(t *testing.T) {
		defer pruneTables(t, postgresDB.db)
		testutils.LoadFixtures(t, postgresDB.db, "fixtures/transactions")

		txHash := testutils.RevChainhash(t, "3296b4cca1c8b1de10b7d4a259963450bf0ed8b481f1fc79e2fb956cfe42242f")
		require.NoError(t, err)

		records, err := postgresDB.GetSeenSinceLastMined(ctx, 1*time.Hour, 5*time.Minute, 2, 0)
		require.NoError(t, err)

		require.Equal(t, 1, len(records))
		require.Equal(t, txHash, records[0].Hash)
		require.Equal(t, records[0].LockedBy, postgresDB.hostname)

		records, err = postgresDB.GetSeenSinceLastMined(ctx, 1*time.Hour, 3*time.Hour, 2, 100)
		require.NoError(t, err)
		require.Equal(t, 0, len(records))
	})

	t.Run("get stats", func(t *testing.T) {
		defer pruneTables(t, postgresDB.db)
		testutils.LoadFixtures(t, postgresDB.db, "fixtures/get_stats")

		res, err := postgresDB.GetStats(ctx, time.Date(2023, 1, 1, 1, 0, 0, 0, time.UTC), 10*time.Minute, 20*time.Minute)
		require.NoError(t, err)

		require.Equal(t, int64(1), res.StatusSeenOnNetwork)
		require.Equal(t, int64(1), res.StatusAcceptedByNetwork)
		require.Equal(t, int64(2), res.StatusAnnouncedToNetwork)
		require.Equal(t, int64(2), res.StatusMined)
		require.Equal(t, int64(0), res.StatusStored)
		require.Equal(t, int64(0), res.StatusRequestedByNetwork)
		require.Equal(t, int64(0), res.StatusSentToNetwork)
		require.Equal(t, int64(1), res.StatusRejected)
		require.Equal(t, int64(0), res.StatusSeenInOrphanMempool)
		require.Equal(t, int64(1), res.StatusDoubleSpendAttempted)
		require.Equal(t, int64(2), res.StatusNotFinal)
		require.Equal(t, int64(2), res.StatusNotSeen)
		require.Equal(t, int64(6), res.StatusMinedTotal)
		require.Equal(t, int64(2), res.StatusSeenOnNetworkTotal)
	})
}
