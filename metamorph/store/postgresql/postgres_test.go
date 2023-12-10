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
			"POSTGRES_PASSWORD=arcpass",
			"POSTGRES_USER=arcuser",
			"POSTGRES_DB=main",
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

	dbInfo = fmt.Sprintf("host=localhost port=%s user=arcuser password=arcpass dbname=main sslmode=disable", hostPort)
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
	t.Run("get/set/del/set unlocked", func(t *testing.T) {
		mined := *minedData
		err = postgresDB.Set(ctx, minedHash[:], &mined)
		require.NoError(t, err)

		dataReturned, err := postgresDB.Get(ctx, minedHash[:])
		require.NoError(t, err)
		require.Equal(t, dataReturned, &mined)

		err = postgresDB.SetUnlocked(ctx, []*chainhash.Hash{minedHash})
		require.NoError(t, err)

		dataReturned, err = postgresDB.Get(ctx, minedHash[:])
		require.NoError(t, err)

		mined.LockedBy = "NONE"
		require.Equal(t, dataReturned, &mined)

		err = postgresDB.Del(ctx, minedHash[:])
		require.NoError(t, err)
	})

	t.Run("set unlocked by name", func(t *testing.T) {
		mined := *minedData
		err = postgresDB.Set(ctx, minedHash[:], &mined)
		require.NoError(t, err)

		rows, err := postgresDB.SetUnlockedByName(ctx, "metamorph-1")
		require.NoError(t, err)
		require.Equal(t, 1, rows)

		dataReturned, err := postgresDB.Get(ctx, minedHash[:])
		require.NoError(t, err)
		mined.LockedBy = "NONE"
		require.Equal(t, dataReturned, &mined)

		err = postgresDB.Del(ctx, minedHash[:])
		require.NoError(t, err)
	})

	t.Run("get unmined", func(t *testing.T) {
		unmined := *unminedData
		err = postgresDB.Set(ctx, unminedHash[:], &unmined)
		require.NoError(t, err)

		err := postgresDB.GetUnmined(ctx, func(record *store.StoreData) {
			require.Equal(t, &unmined, record)
		})
		require.NoError(t, err)

		err = postgresDB.Del(ctx, unminedHash[:])
		require.NoError(t, err)
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

	t.Run("is centralised", func(t *testing.T) {
		require.True(t, postgresDB.IsCentralised())
	})
}
