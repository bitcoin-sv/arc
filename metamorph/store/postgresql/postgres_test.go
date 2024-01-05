package postgresql

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/metamorph/store"
	"github.com/bitcoin-sv/arc/testdata"
	"github.com/go-testfixtures/testfixtures/v3"
	"github.com/golang-migrate/migrate/v4"
	migratepostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
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
		MerkleProof:   true,
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
		mined := *minedData
		err = postgresDB.Set(ctx, minedHash[:], &mined)
		require.NoError(t, err)

		dataReturned, err := postgresDB.Get(ctx, minedHash[:])
		require.NoError(t, err)
		require.Equal(t, dataReturned, &mined)

		err = postgresDB.Del(ctx, minedHash[:])
		require.NoError(t, err)
	})

	t.Run("remove callback url", func(t *testing.T) {
		err = postgresDB.RemoveCallbacker(ctx, minedHash)
		require.NoError(t, err)
	})

	t.Run("get unmined", func(t *testing.T) {
		fmt.Println(dbInfo)

		expectedHash, err := chainhash.NewHashFromStr("57438c4340b9a5e0d77120d999765589048f6f2dd49a6325cdf14356fc4cc012")
		require.NoError(t, err)
		expectedHashFound := false
		err = postgresDB.GetUnmined(ctx, func(record *store.StoreData) {
			expectedHashFound = true
			require.Equal(t, expectedHash, record.Hash)
		})
		require.NoError(t, err)
		require.True(t, expectedHashFound)

		dataReturned, err := postgresDB.Get(ctx, expectedHash[:])
		require.NoError(t, err)
		require.Equal(t, "metamorph-1", dataReturned.LockedBy)

		err = postgresDB.Del(ctx, unminedHash[:])
		require.NoError(t, err)
	})

	t.Run("set unlocked", func(t *testing.T) {
		fmt.Println(dbInfo)
		chainHash1, err := chainhash.NewHashFromStr("30f409d6951483e4d65a586205f373c2f72431ade49abb6f143e82fc53ea6cb1")
		require.NoError(t, err)
		chainHash2, err := chainhash.NewHashFromStr("aaf6dcac409778330982371560bd4e4f7064e4154aa1e66a3e3d8946b7f576ee")
		require.NoError(t, err)
		chainHash3, err := chainhash.NewHashFromStr("cd8e0640fadac1f9ed854174558e37a54ede58bc85f49bf01041348c215b0b3e")
		require.NoError(t, err)
		chainHash4, err := chainhash.NewHashFromStr("f0499acf63b00e042e6be506bfe67c3c856b95129252aeb5820e46c5878c3a21")
		require.NoError(t, err)

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
		fmt.Println(dbInfo)
		rows, err := postgresDB.SetUnlockedByName(ctx, "metamorph-3")
		require.NoError(t, err)
		require.Equal(t, 2, rows)
	})

	t.Run("update status", func(t *testing.T) {
		unmined := *unminedData
		err = postgresDB.Set(ctx, unminedHash[:], &unmined)
		require.NoError(t, err)

		err := postgresDB.UpdateStatus(ctx, unminedHash, metamorph_api.Status_REJECTED, "missing inputs")
		require.NoError(t, err)

		dataReturned, err := postgresDB.Get(ctx, unminedHash[:])
		require.NoError(t, err)
		unmined.Status = metamorph_api.Status_REJECTED
		unmined.RejectReason = "missing inputs"
		require.Equal(t, dataReturned, &unmined)

		err = postgresDB.Del(ctx, unminedHash[:])
		require.NoError(t, err)
	})

	t.Run("update mined", func(t *testing.T) {
		unmined := *unminedData
		err = postgresDB.Set(ctx, unminedHash[:], &unmined)
		require.NoError(t, err)

		err := postgresDB.UpdateMined(ctx, unminedHash, testdata.Block1Hash, 100)
		require.NoError(t, err)

		dataReturned, err := postgresDB.Get(ctx, unminedHash[:])
		require.NoError(t, err)
		unmined.Status = metamorph_api.Status_MINED
		unmined.MinedAt = now
		unmined.BlockHeight = 100
		unmined.BlockHash = testdata.Block1Hash
		require.Equal(t, dataReturned, &unmined)

		err = postgresDB.Del(ctx, unminedHash[:])
		require.NoError(t, err)
	})

	t.Run("get/set block processed", func(t *testing.T) {
		err = postgresDB.SetBlockProcessed(ctx, testdata.Block1Hash)
		require.NoError(t, err)

		processedAt, err := postgresDB.GetBlockProcessed(ctx, testdata.Block1Hash)
		require.NoError(t, err)

		require.Equal(t, &now, processedAt)
	})
}
