package postgresql

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/bitcoin-sv/arc/internal/testdata"
	"github.com/go-testfixtures/testfixtures/v3"
	"github.com/golang-migrate/migrate/v4"
	migratepostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	postgresPort   = "5432"
	migrationsPath = "file://migrations"
	dbName         = "main_test"
	dbUsername     = "arcuser"
	dbPassword     = "arcpass"
)

var dbInfo string

func revChainhash(t *testing.T, hashString string) *chainhash.Hash {
	hash, err := hex.DecodeString(hashString)
	require.NoError(t, err)
	txHash, err := chainhash.NewHash(hash)
	require.NoError(t, err)

	return txHash
}

func TestMain(m *testing.M) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("failed to create pool: %v", err)
	}

	port := "5433"
	opts := dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "15.4",
		Env: []string{
			fmt.Sprintf("POSTGRES_PASSWORD=%s", dbPassword),
			fmt.Sprintf("POSTGRES_USER=%s", dbUsername),
			fmt.Sprintf("POSTGRES_DB=%s", dbName),
			"listen_addresses = '*'",
		},
		ExposedPorts: []string{"5432"},
		PortBindings: map[docker.Port][]docker.PortBinding{
			postgresPort: {
				{HostIP: "0.0.0.0", HostPort: port},
			},
		},
	}

	resource, err := pool.RunWithOptions(&opts, func(config *docker.HostConfig) {
		// set AutoRemove to true so that stopped container goes away by itself
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{
			Name: "no",
		}
		config.Tmpfs = map[string]string{
			"/var/lib/postgresql/data": "",
		}
	})
	if err != nil {
		log.Fatalf("failed to create resource: %v", err)
	}

	hostPort := resource.GetPort("5432/tcp")

	dbInfo = fmt.Sprintf("host=localhost port=%s user=%s password=%s dbname=%s sslmode=disable", hostPort, dbUsername, dbPassword, dbName)
	var postgresDB *PostgreSQL
	err = pool.Retry(func() error {
		postgresDB, err = New(dbInfo, "localhost", 10, 10)
		if err != nil {
			return err
		}
		return postgresDB.db.Ping()
	})
	if err != nil {
		log.Fatalf("failed to connect to docker: %s", err)
	}

	driver, err := migratepostgres.WithInstance(postgresDB.db, &migratepostgres.Config{
		MigrationsTable: "metamorph",
	})
	if err != nil {
		log.Fatalf("failed to create driver: %v", err)
	}

	migrations, err := migrate.NewWithDatabaseInstance(
		migrationsPath,
		"postgres", driver)
	if err != nil {
		log.Fatalf("failed to initialize migrate instance: %v", err)
	}
	err = migrations.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatalf("failed to initialize migrate instance: %v", err)
	}

	fixtures, err := testfixtures.New(
		testfixtures.Database(postgresDB.db),
		testfixtures.Dialect("postgresql"),
		testfixtures.Directory("fixtures"), // The directory containing the YAML files
	)
	if err != nil {
		log.Fatalf("failed to create fixtures: %v", err)
	}

	err = fixtures.Load()
	if err != nil {
		log.Fatalf("failed to load fixtures: %v", err)
	}
	code := m.Run()

	err = pool.Purge(resource)
	if err != nil {
		log.Fatalf("failed to purge pool: %v", err)
	}

	os.Exit(code)
}

func loadFixtures(db *sql.DB, path string) error {
	fixtures, err := testfixtures.New(
		testfixtures.Database(db),
		testfixtures.Dialect("postgresql"),
		testfixtures.Directory(path), // The directory containing the YAML files
	)
	if err != nil {
		log.Fatalf("failed to create fixtures: %v", err)
	}

	err = fixtures.Load()
	if err != nil {
		return fmt.Errorf("failed to load fixtures: %v", err)
	}

	return nil
}

func pruneTables(db *sql.DB) error {
	_, err := db.Exec("TRUNCATE TABLE metamorph.transactions;")
	if err != nil {
		return err
	}

	return nil
}

func TestPostgresDB(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	now := time.Date(2023, 10, 1, 14, 25, 0, 0, time.UTC)
	minedHash := testdata.TX1Hash
	minedData := &store.StoreData{
		RawTx:         make([]byte, 0),
		StoredAt:      now,
		AnnouncedAt:   time.Date(2023, 10, 1, 12, 5, 0, 0, time.UTC),
		MinedAt:       time.Date(2023, 10, 1, 12, 10, 0, 0, time.UTC),
		Hash:          minedHash,
		Status:        metamorph_api.Status_MINED,
		BlockHeight:   100,
		BlockHash:     testdata.Block1Hash,
		CallbackUrl:   "http://callback.example.com",
		CallbackToken: "12345",
		RejectReason:  "not rejected",
		LockedBy:      "metamorph-1",
	}

	unminedHash := testdata.TX1Hash
	unminedData := &store.StoreData{
		RawTx:       make([]byte, 0),
		StoredAt:    now,
		AnnouncedAt: time.Date(2023, 10, 1, 12, 5, 0, 0, time.UTC),
		Hash:        unminedHash,
		Status:      metamorph_api.Status_SENT_TO_NETWORK,
		LockedBy:    "metamorph-1",
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
		defer require.NoError(t, pruneTables(postgresDB.db))

		mined := *minedData
		err = postgresDB.Set(ctx, &mined)
		require.NoError(t, err)

		dataReturned, err := postgresDB.Get(ctx, minedHash[:])
		require.NoError(t, err)
		require.Equal(t, dataReturned, &mined)

		mined.LastSubmittedAt = time.Date(2024, 5, 31, 15, 16, 0, 0, time.UTC)
		err = postgresDB.Set(ctx, &mined)
		require.NoError(t, err)

		dataReturned2, err := postgresDB.Get(ctx, minedHash[:])
		require.NoError(t, err)
		require.Equal(t, time.Date(2024, 5, 31, 15, 16, 0, 0, time.UTC), dataReturned2.LastSubmittedAt)

		err = postgresDB.Del(ctx, minedHash[:])
		require.NoError(t, err)

		_, err = postgresDB.Get(ctx, minedHash[:])
		require.True(t, errors.Is(err, store.ErrNotFound))

		_, err = postgresDB.Get(ctx, []byte("not to be found"))
		require.True(t, errors.Is(err, store.ErrNotFound))
	})

	t.Run("get raw txs", func(t *testing.T) {
		defer require.NoError(t, pruneTables(postgresDB.db))

		require.NoError(t, loadFixtures(postgresDB.db, "fixtures/get_rawtxs"))

		hash1 := "cd3d2f97dfc0cdb6a07ec4b72df5e1794c9553ff2f62d90ed4add047e8088853"
		hash2 := "21132d32cb5411c058bb4391f24f6a36ed9b810df851d0e36cac514fd03d6b4e"
		hash3 := "3e0b5b218c344110f09bf485bc58de4ea5378e55744185edf9c1dafa40068ecd"

		hashes := make([][]byte, 0)
		hashes = append(hashes, revChainhash(t, hash1).CloneBytes())
		hashes = append(hashes, revChainhash(t, hash2).CloneBytes())
		hashes = append(hashes, revChainhash(t, hash3).CloneBytes())

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

	t.Run("set bulk", func(t *testing.T) {
		defer require.NoError(t, pruneTables(postgresDB.db))

		require.NoError(t, loadFixtures(postgresDB.db, "fixtures/set_bulk"))

		hash2 := revChainhash(t, "cd3d2f97dfc0cdb6a07ec4b72df5e1794c9553ff2f62d90ed4add047e8088853") // hash already existing in db - no update expected

		data := []*store.StoreData{
			{
				RawTx:             testdata.TX1Raw.Bytes(),
				StoredAt:          now,
				Hash:              testdata.TX1Hash,
				Status:            metamorph_api.Status_STORED,
				CallbackUrl:       "callback.example.com",
				FullStatusUpdates: false,
				CallbackToken:     "1234",
				LastSubmittedAt:   now,
				LockedBy:          "metamorph-1",
			},
			{
				RawTx:             testdata.TX6Raw.Bytes(),
				StoredAt:          now,
				Hash:              testdata.TX6Hash,
				Status:            metamorph_api.Status_STORED,
				CallbackUrl:       "callback-2.example.com",
				FullStatusUpdates: true,
				CallbackToken:     "5678",
				LastSubmittedAt:   now,
				LockedBy:          "metamorph-1",
			},
			{
				RawTx:             testdata.TX6Raw.Bytes(),
				StoredAt:          now,
				Hash:              hash2,
				Status:            metamorph_api.Status_STORED,
				CallbackUrl:       "callback-3.example.com",
				FullStatusUpdates: true,
				CallbackToken:     "5678",
				LastSubmittedAt:   now,
				LockedBy:          "metamorph-1",
			},
		}

		err = postgresDB.SetBulk(ctx, data)
		require.NoError(t, err)

		data0, err := postgresDB.Get(ctx, testdata.TX1Hash[:])
		require.NoError(t, err)
		require.Equal(t, data[0], data0)

		data1, err := postgresDB.Get(ctx, testdata.TX6Hash[:])
		require.NoError(t, err)
		require.Equal(t, data[1], data1)

		data2, err := postgresDB.Get(ctx, hash2[:])
		require.NoError(t, err)
		require.NotEqual(t, data[2], data2)
		require.Equal(t, metamorph_api.Status_SENT_TO_NETWORK, data2.Status)
		require.Equal(t, "metamorph-3", data2.LockedBy)
		require.Equal(t, now, data2.LastSubmittedAt)
	})

	t.Run("get unmined", func(t *testing.T) {
		defer require.NoError(t, pruneTables(postgresDB.db))

		require.NoError(t, loadFixtures(postgresDB.db, "fixtures"))

		// locked by metamorph-1
		expectedHash0 := revChainhash(t, "3e0b5b218c344110f09bf485bc58de4ea5378e55744185edf9c1dafa40068ecd")
		// offset 0
		records, err := postgresDB.GetUnmined(ctx, time.Date(2023, 1, 1, 1, 0, 0, 0, time.UTC), 1, 0)
		require.NoError(t, err)
		require.Equal(t, expectedHash0, records[0].Hash)

		// locked by metamorph-1
		expectedHash1 := revChainhash(t, "b16cea53fc823e146fbb9ae4ad3124f7c273f30562585ad6e4831495d609f430")
		// offset 1
		records, err = postgresDB.GetUnmined(ctx, time.Date(2023, 1, 1, 1, 0, 0, 0, time.UTC), 1, 1)
		require.NoError(t, err)
		require.Equal(t, expectedHash1, records[0].Hash)
	})

	t.Run("set locked by", func(t *testing.T) {
		defer require.NoError(t, pruneTables(postgresDB.db))

		require.NoError(t, loadFixtures(postgresDB.db, "fixtures"))

		err := postgresDB.SetLocked(ctx, time.Date(2023, 9, 15, 1, 0, 0, 0, time.UTC), 2)
		require.NoError(t, err)

		// locked by NONE
		expectedHash2 := revChainhash(t, "12c04cfc5643f1cd25639ad42d6f8f0489557699d92071d7e0a5b940438c4357")

		// locked by NONE
		expectedHash3 := revChainhash(t, "78d66c8391ff5e4a65b494e39645facb420b744f77f3f3b83a3aa8573282176e")

		// check if previously unlocked tx has b
		// locked by NONE
		expectedHash4 := revChainhash(t, "319b5eb9d99084b72002640d1445f49b8c83539260a7e5b2cbb16c1d2954a743")

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
		defer require.NoError(t, pruneTables(postgresDB.db))

		require.NoError(t, loadFixtures(postgresDB.db, "fixtures/set_unlocked_by_name"))

		rows, err := postgresDB.SetUnlockedByName(ctx, "metamorph-3")
		require.NoError(t, err)
		require.Equal(t, int64(4), rows)

		hash1 := revChainhash(t, "cd3d2f97dfc0cdb6a07ec4b72df5e1794c9553ff2f62d90ed4add047e8088853")
		hash1Data, err := postgresDB.Get(ctx, hash1[:])
		require.NoError(t, err)
		require.Equal(t, "NONE", hash1Data.LockedBy)

		hash2 := revChainhash(t, "21132d32cb5411c058bb4391f24f6a36ed9b810df851d0e36cac514fd03d6b4e")
		hash2Data, err := postgresDB.Get(ctx, hash2[:])
		require.NoError(t, err)
		require.Equal(t, "NONE", hash2Data.LockedBy)

		hash3 := revChainhash(t, "f791ec50447e3001b9348930659527ea92dee506e9950014bcc7c5b146e2417f")
		hash3Data, err := postgresDB.Get(ctx, hash3[:])
		require.NoError(t, err)
		require.Equal(t, "NONE", hash3Data.LockedBy)

		hash4 := revChainhash(t, "89714f129748e5176a07fc4eb89cf27a9e60340117e6b56bb742acb2873f8140")
		hash4Data, err := postgresDB.Get(ctx, hash4[:])
		require.NoError(t, err)
		require.Equal(t, "NONE", hash4Data.LockedBy)
	})

	t.Run("update status", func(t *testing.T) {
		defer require.NoError(t, pruneTables(postgresDB.db))

		require.NoError(t, loadFixtures(postgresDB.db, "fixtures/update_status"))

		updates := []store.UpdateStatus{
			{
				Hash:   *revChainhash(t, "cd3d2f97dfc0cdb6a07ec4b72df5e1794c9553ff2f62d90ed4add047e8088853"), // update expected
				Status: metamorph_api.Status_ACCEPTED_BY_NETWORK,
			},
			{
				Hash:   *revChainhash(t, "21132d32cb5411c058bb4391f24f6a36ed9b810df851d0e36cac514fd03d6b4e"), // update not expected - old status = new status
				Status: metamorph_api.Status_REQUESTED_BY_NETWORK,
			},
			{
				Hash:         *revChainhash(t, "b16cea53fc823e146fbb9ae4ad3124f7c273f30562585ad6e4831495d609f430"), // update expected
				Status:       metamorph_api.Status_REJECTED,
				RejectReason: "missing inputs",
			},
			{
				Hash:   *revChainhash(t, "ee76f5b746893d3e6ae6a14a15e464704f4ebd601537820933789740acdcf6aa"), // update expected
				Status: metamorph_api.Status_SEEN_ON_NETWORK,
			},
			{
				Hash:   *revChainhash(t, "3e0b5b218c344110f09bf485bc58de4ea5378e55744185edf9c1dafa40068ecd"), // update not expected - status is mined
				Status: metamorph_api.Status_SENT_TO_NETWORK,
			},
			{
				Hash:   *revChainhash(t, "7809b730cbe7bb723f299a4e481fb5165f31175876392a54cde85569a18cc75f"), // update not expected - old status > new status
				Status: metamorph_api.Status_SENT_TO_NETWORK,
			},
			{
				Hash:   *revChainhash(t, "3ce1e0c6cbbbe2118c3f80d2e6899d2d487f319ef0923feb61f3d26335b2225c"), // update not expected - hash non-existent in db
				Status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
			},
			{
				Hash:   *revChainhash(t, "7e3350ca12a0dd9375540e13637b02e054a3436336e9d6b82fe7f2b23c710002"), // update not expected - hash non-existent in db
				Status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
			},
		}

		statusUpdates, err := postgresDB.UpdateStatusBulk(ctx, updates)
		require.NoError(t, err)
		require.Len(t, statusUpdates, 3)

		require.Equal(t, metamorph_api.Status_ACCEPTED_BY_NETWORK, statusUpdates[0].Status)
		require.Equal(t, *revChainhash(t, "cd3d2f97dfc0cdb6a07ec4b72df5e1794c9553ff2f62d90ed4add047e8088853"), *statusUpdates[0].Hash)

		require.Equal(t, metamorph_api.Status_REJECTED, statusUpdates[1].Status)
		require.Equal(t, "missing inputs", statusUpdates[1].RejectReason)
		require.Equal(t, *revChainhash(t, "b16cea53fc823e146fbb9ae4ad3124f7c273f30562585ad6e4831495d609f430"), *statusUpdates[1].Hash)

		require.Equal(t, metamorph_api.Status_SEEN_ON_NETWORK, statusUpdates[2].Status)
		require.Equal(t, *revChainhash(t, "ee76f5b746893d3e6ae6a14a15e464704f4ebd601537820933789740acdcf6aa"), *statusUpdates[2].Hash)

		returnedDataRequested, err := postgresDB.Get(ctx, revChainhash(t, "7809b730cbe7bb723f299a4e481fb5165f31175876392a54cde85569a18cc75f")[:])
		require.NoError(t, err)
		require.Equal(t, metamorph_api.Status_ACCEPTED_BY_NETWORK, returnedDataRequested.Status)

		statusUpdates, err = postgresDB.UpdateStatusBulk(ctx, updates)
		require.NoError(t, err)
		require.Len(t, statusUpdates, 0)
	})

	t.Run("update mined", func(t *testing.T) {
		defer require.NoError(t, pruneTables(postgresDB.db))

		require.NoError(t, loadFixtures(postgresDB.db, "fixtures"))

		unmined := *unminedData
		err = postgresDB.Set(ctx, &unmined)
		require.NoError(t, err)

		chainHash2 := revChainhash(t, "ee76f5b746893d3e6ae6a14a15e464704f4ebd601537820933789740acdcf6aa")

		txBlocks := &blocktx_api.TransactionBlocks{TransactionBlocks: []*blocktx_api.TransactionBlock{
			{
				BlockHash:       testdata.Block1Hash[:],
				BlockHeight:     100,
				TransactionHash: unminedHash[:],
				MerklePath:      "merkle-path-1",
			},
			{
				BlockHash:       testdata.Block1Hash[:],
				BlockHeight:     100,
				TransactionHash: chainHash2[:],
				MerklePath:      "merkle-path-2",
			},
			{
				BlockHash:       testdata.Block1Hash[:],
				BlockHeight:     100,
				TransactionHash: testdata.TX3Hash[:], // hash non-existent in db
				MerklePath:      "merkle-path-3",
			},
		}}

		updated, err := postgresDB.UpdateMined(ctx, txBlocks)
		require.NoError(t, err)
		require.Len(t, updated, 2)

		require.True(t, chainHash2.IsEqual(updated[0].Hash))
		require.True(t, testdata.Block1Hash.IsEqual(updated[0].BlockHash))
		require.Equal(t, "merkle-path-2", updated[0].MerklePath)

		require.True(t, unminedHash.IsEqual(updated[1].Hash))
		require.True(t, testdata.Block1Hash.IsEqual(updated[1].BlockHash))
		require.Equal(t, "merkle-path-1", updated[1].MerklePath)

		dataReturned, err := postgresDB.Get(ctx, unminedHash[:])
		require.NoError(t, err)
		unmined.Status = metamorph_api.Status_MINED
		unmined.MinedAt = now
		unmined.BlockHeight = 100
		unmined.BlockHash = testdata.Block1Hash
		unmined.MerklePath = "merkle-path-1"
		require.Equal(t, dataReturned, &unmined)

		updated, err = postgresDB.UpdateMined(ctx, &blocktx_api.TransactionBlocks{})
		require.NoError(t, err)
		require.Len(t, updated, 0)
		require.Len(t, updated, 0)

		updated, err = postgresDB.UpdateMined(ctx, nil)
		require.NoError(t, err)
		require.Nil(t, updated)
	})

	t.Run("update mined - missing block info", func(t *testing.T) {
		defer require.NoError(t, pruneTables(postgresDB.db))

		unmined := *unminedData
		err = postgresDB.Set(ctx, &unmined)
		require.NoError(t, err)
		txBlocks := &blocktx_api.TransactionBlocks{TransactionBlocks: []*blocktx_api.TransactionBlock{{
			BlockHash:       nil,
			BlockHeight:     0,
			TransactionHash: unminedHash[:],
			MerklePath:      "",
		}}}

		_, err := postgresDB.UpdateMined(ctx, txBlocks)
		require.NoError(t, err)

		dataReturned, err := postgresDB.Get(ctx, unminedHash[:])
		require.NoError(t, err)
		unmined.Status = metamorph_api.Status_MINED
		unmined.MinedAt = now
		unmined.BlockHeight = 0
		unmined.BlockHash = nil
		require.Equal(t, dataReturned, &unmined)
	})

	t.Run("clear data", func(t *testing.T) {
		defer require.NoError(t, pruneTables(postgresDB.db))

		require.NoError(t, loadFixtures(postgresDB.db, "fixtures"))

		res, err := postgresDB.ClearData(ctx, 14)
		require.NoError(t, err)
		require.Equal(t, int64(5), res)

		var numberOfRemainingTxs int
		err = postgresDB.db.QueryRowContext(ctx, "SELECT count(*) FROM metamorph.transactions;").Scan(&numberOfRemainingTxs)
		require.NoError(t, err)
		require.Equal(t, 10, numberOfRemainingTxs)
	})

	t.Run("get seen on network txs", func(t *testing.T) {
		defer require.NoError(t, pruneTables(postgresDB.db))

		require.NoError(t, loadFixtures(postgresDB.db, "fixtures"))

		txHash := revChainhash(t, "855b2aea1420df52a561fe851297653739677b14c89c0a08e3f70e1942bcb10f")
		require.NoError(t, err)

		records, err := postgresDB.GetSeenOnNetwork(ctx, time.Date(2023, 1, 1, 1, 0, 0, 0, time.UTC), time.Date(2023, 1, 1, 3, 0, 0, 0, time.UTC), 2, 0)
		require.NoError(t, err)

		require.Equal(t, txHash, records[0].Hash)
		require.Equal(t, 1, len(records))
		require.Equal(t, records[0].LockedBy, postgresDB.hostname)
	})

	t.Run("get stats", func(t *testing.T) {
		defer require.NoError(t, pruneTables(postgresDB.db))

		require.NoError(t, loadFixtures(postgresDB.db, "fixtures/get_stats"))

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
		require.Equal(t, int64(1), res.StatusNotMined)
		require.Equal(t, int64(2), res.StatusNotSeen)
		require.Equal(t, int64(6), res.StatusMinedTotal)
		require.Equal(t, int64(2), res.StatusSeenOnNetworkTotal)
	})
}
