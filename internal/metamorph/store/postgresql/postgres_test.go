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

	"github.com/bitcoin-sv/arc/internal/testdata"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
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

var (
	dbInfo string
)

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
	now := time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC)
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

	hash1, err := hex.DecodeString("b16cea53fc823e146fbb9ae4ad3124f7c273f30562585ad6e4831495d609f430") // sent
	require.NoError(t, err)
	chainHash1, err := chainhash.NewHash(hash1)
	require.NoError(t, err)

	hash2, err := hex.DecodeString("ee76f5b746893d3e6ae6a14a15e464704f4ebd601537820933789740acdcf6aa") // seen
	require.NoError(t, err)
	chainHash2, err := chainhash.NewHash(hash2)
	require.NoError(t, err)

	hash3, err := hex.DecodeString("3e0b5b218c344110f09bf485bc58de4ea5378e55744185edf9c1dafa40068ecd") // announced
	require.NoError(t, err)
	chainHash3, err := chainhash.NewHash(hash3)
	require.NoError(t, err)

	hash4, err := hex.DecodeString("213a8c87c5460e82b5ae529212956b853c7ce6bf06e56b2e040eb063cf9a49f0") // mined
	require.NoError(t, err)
	chainHash4, err := chainhash.NewHash(hash4)
	require.NoError(t, err)

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
		err = postgresDB.Set(ctx, minedHash[:], &mined)
		require.NoError(t, err)

		dataReturned, err := postgresDB.Get(ctx, minedHash[:])
		require.NoError(t, err)
		require.Equal(t, dataReturned, &mined)

		mined.InsertedAtNum = 1234
		err = postgresDB.Set(ctx, minedHash[:], &mined)
		require.NoError(t, err)

		dataReturned2, err := postgresDB.Get(ctx, minedHash[:])
		require.NoError(t, err)
		require.Equal(t, 1234, dataReturned2.InsertedAtNum)

		err = postgresDB.Del(ctx, minedHash[:])
		require.NoError(t, err)

		_, err = postgresDB.Get(ctx, minedHash[:])
		require.True(t, errors.Is(err, store.ErrNotFound))

		_, err = postgresDB.Get(ctx, []byte("not to be found"))
		require.True(t, errors.Is(err, store.ErrNotFound))
	})

	t.Run("get unmined", func(t *testing.T) {
		defer require.NoError(t, pruneTables(postgresDB.db))

		require.NoError(t, loadFixtures(postgresDB.db, "fixtures"))

		// locked by metamorph-1
		expectedHash0, err := chainhash.NewHashFromStr("cd8e0640fadac1f9ed854174558e37a54ede58bc85f49bf01041348c215b0b3e")
		require.NoError(t, err)
		// offset 0
		records, err := postgresDB.GetUnmined(ctx, time.Date(2023, 1, 1, 1, 0, 0, 0, time.UTC), 1, 0)
		require.NoError(t, err)
		require.Equal(t, expectedHash0, records[0].Hash)

		// locked by metamorph-1
		expectedHash1, err := chainhash.NewHashFromStr("30f409d6951483e4d65a586205f373c2f72431ade49abb6f143e82fc53ea6cb1")
		require.NoError(t, err)
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
		expectedHash2, err := chainhash.NewHashFromStr("57438c4340b9a5e0d77120d999765589048f6f2dd49a6325cdf14356fc4cc012")
		require.NoError(t, err)

		// locked by NONE
		expectedHash3, err := chainhash.NewHashFromStr("6e17823257a83a3ab8f3f3774f740b42cbfa4596e394b4654a5eff91836cd678")
		require.NoError(t, err)

		// check if previously unlocked tx has b
		// locked by NONE
		expectedHash4, err := chainhash.NewHashFromStr("43a754291d6cb1cbb2e5a7609253838c9bf445140d640220b78490d9b95e9b31")
		require.NoError(t, err)

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

	t.Run("set unlocked", func(t *testing.T) {
		defer require.NoError(t, pruneTables(postgresDB.db))

		require.NoError(t, loadFixtures(postgresDB.db, "fixtures"))

		err = postgresDB.SetUnlocked(ctx, []*chainhash.Hash{chainHash1, chainHash2, chainHash3, chainHash4})
		require.NoError(t, err)

		dataReturned1, err := postgresDB.Get(ctx, chainHash1[:])
		require.NoError(t, err)
		require.Equal(t, "NONE", dataReturned1.LockedBy)

		dataReturned2, err := postgresDB.Get(ctx, chainHash2[:])
		require.NoError(t, err)
		require.Equal(t, "NONE", dataReturned2.LockedBy)
		dataReturned3, err := postgresDB.Get(ctx, chainHash3[:])
		require.NoError(t, err)
		require.Equal(t, "NONE", dataReturned3.LockedBy)
		dataReturned4, err := postgresDB.Get(ctx, chainHash4[:])
		require.NoError(t, err)
		require.Equal(t, "NONE", dataReturned4.LockedBy)
	})

	t.Run("set unlocked by name", func(t *testing.T) {
		defer require.NoError(t, pruneTables(postgresDB.db))

		require.NoError(t, loadFixtures(postgresDB.db, "fixtures"))

		rows, err := postgresDB.SetUnlockedByName(ctx, "metamorph-3")
		require.NoError(t, err)
		require.Equal(t, int64(2), rows)
	})

	t.Run("update status", func(t *testing.T) {
		defer require.NoError(t, pruneTables(postgresDB.db))

		require.NoError(t, loadFixtures(postgresDB.db, "fixtures"))

		tx1Data := &store.StoreData{
			RawTx:  testdata.TX1RawBytes,
			Hash:   testdata.TX1Hash,
			Status: metamorph_api.Status_STORED,
		}
		err = postgresDB.Set(ctx, testdata.TX1Hash[:], tx1Data)
		require.NoError(t, err)

		tx6Data := &store.StoreData{
			RawTx:  testdata.TX6RawBytes,
			Hash:   testdata.TX6Hash,
			Status: metamorph_api.Status_STORED,
		}

		err = postgresDB.Set(ctx, testdata.TX6Hash[:], tx6Data)
		require.NoError(t, err)

		// will now be updated even though it is locked by metamorph-3
		metamorph3Hash, err := chainhash.NewHashFromStr("538808e847d0add40ed9622fff53954c79e1f52db7c47ea0b6cdc0df972f3dcd")
		require.NoError(t, err)

		// will not be updated because has already status seen on network
		seenOnNetworkHash, err := chainhash.NewHashFromStr("aaf6dcac409778330982371560bd4e4f7064e4154aa1e66a3e3d8946b7f576ee")
		require.NoError(t, err)

		// will not be updated because has already status mined
		minedHash, err := chainhash.NewHashFromStr("1d7b3ab38d773d5cf6815dce868087b61783a1778b97a5c1f018951f614548a2")
		require.NoError(t, err)

		updates := []store.UpdateStatus{
			{
				Hash:         *testdata.TX1Hash,
				Status:       metamorph_api.Status_REQUESTED_BY_NETWORK,
				RejectReason: "",
			},
			{
				Hash:         *testdata.TX6Hash,
				Status:       metamorph_api.Status_REJECTED,
				RejectReason: "missing inputs",
			},
			{
				Hash:   *testdata.TX3Hash, // hash non-existent in db
				Status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
			},
			{
				Hash:   *testdata.TX4Hash, // hash non-existent in db
				Status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
			},
			{
				Hash:   *metamorph3Hash,
				Status: metamorph_api.Status_MINED,
			},
			{
				Hash:   *seenOnNetworkHash,
				Status: metamorph_api.Status_REQUESTED_BY_NETWORK,
			},
			{
				Hash:   *minedHash,
				Status: metamorph_api.Status_SENT_TO_NETWORK,
			},
		}

		statusUpdates, err := postgresDB.UpdateStatusBulk(ctx, updates)
		require.NoError(t, err)
		require.Len(t, statusUpdates, 3)

		assert.Equal(t, metamorph_api.Status_REQUESTED_BY_NETWORK, statusUpdates[1].Status)
		assert.Equal(t, testdata.TX1RawBytes, statusUpdates[1].RawTx)

		assert.Equal(t, metamorph_api.Status_REJECTED, statusUpdates[2].Status)
		assert.Equal(t, "missing inputs", statusUpdates[2].RejectReason)
		assert.Equal(t, testdata.TX6RawBytes, statusUpdates[2].RawTx)

		returnedDataRejected, err := postgresDB.Get(ctx, testdata.TX1Hash[:])
		require.NoError(t, err)
		assert.Equal(t, metamorph_api.Status_REQUESTED_BY_NETWORK, returnedDataRejected.Status)
		assert.Equal(t, "", returnedDataRejected.RejectReason)
		assert.Equal(t, testdata.TX1RawBytes, returnedDataRejected.RawTx)

		returnedDataRequested, err := postgresDB.Get(ctx, testdata.TX6Hash[:])
		require.NoError(t, err)
		assert.Equal(t, metamorph_api.Status_REJECTED, returnedDataRequested.Status)
		assert.Equal(t, "missing inputs", returnedDataRequested.RejectReason)
		assert.Equal(t, testdata.TX6RawBytes, returnedDataRequested.RawTx)

		returnedDataSeen, err := postgresDB.Get(ctx, seenOnNetworkHash[:])
		require.NoError(t, err)
		assert.Equal(t, metamorph_api.Status_SEEN_ON_NETWORK, returnedDataSeen.Status)

		returnedDataMined, err := postgresDB.Get(ctx, minedHash[:])
		require.NoError(t, err)
		assert.Equal(t, metamorph_api.Status_MINED, returnedDataMined.Status)

		statusUpdates, err = postgresDB.UpdateStatusBulk(ctx, updates)
		require.NoError(t, err)
		require.Len(t, statusUpdates, 2)
	})

	t.Run("update mined", func(t *testing.T) {
		defer require.NoError(t, pruneTables(postgresDB.db))

		require.NoError(t, loadFixtures(postgresDB.db, "fixtures"))

		unmined := *unminedData
		err = postgresDB.Set(ctx, unminedHash[:], &unmined)
		require.NoError(t, err)

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

	})

	t.Run("update mined - missing block info", func(t *testing.T) {
		defer require.NoError(t, pruneTables(postgresDB.db))

		unmined := *unminedData
		err = postgresDB.Set(ctx, unminedHash[:], &unmined)
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
		require.Equal(t, int64(4), res)

		var numberOfRemainingTxs int
		err = postgresDB.db.QueryRowContext(ctx, "SELECT count(*) FROM metamorph.transactions;").Scan(&numberOfRemainingTxs)
		require.NoError(t, err)
		require.Equal(t, 10, numberOfRemainingTxs)
	})
}
