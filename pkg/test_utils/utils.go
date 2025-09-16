package testutils

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/go-testfixtures/testfixtures/v3"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/stretchr/testify/require"

	"github.com/golang-migrate/migrate/v4"
	migratepostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file" //nolint: revive // Required for migrations
)

func MigrateUp(table, path, dbInfo string) error {
	dbConn, err := sql.Open("postgres", dbInfo)
	if err != nil {
		return fmt.Errorf("failed to create db connection: %v", err)
	}
	defer func() {
		_ = dbConn.Close()
	}()

	if err = Retry(dbConn.Ping); err != nil {
		return fmt.Errorf("failed to connect to docker: %s", err)
	}

	driver, err := migratepostgres.WithInstance(dbConn, &migratepostgres.Config{
		MigrationsTable: table,
	})
	if err != nil {
		return fmt.Errorf("failed to create driver: %v", err)
	}

	migrations, err := migrate.NewWithDatabaseInstance(
		path,
		"postgres",
		driver)
	if err != nil {
		return fmt.Errorf("failed to initialize migrate instance: %v", err)
	}

	err = migrations.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to initialize migrate instance: %v", err)
	}

	return nil
}

func Retry(op func() error) error {
	bo := backoff.NewExponentialBackOff()
	bo.MaxInterval = time.Second * 5
	bo.MaxElapsedTime = time.Minute
	if err := backoff.Retry(op, bo); err != nil {
		if bo.NextBackOff() == backoff.Stop {
			return fmt.Errorf("reached retry deadline: %w", err)
		}

		return err
	}

	return nil
}

func LoadFixtures(t testing.TB, db *sql.DB, path string) {
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

func PruneTables(t testing.TB, db *sql.DB, tables ...string) {
	t.Helper()

	for _, tab := range tables {
		_, err := db.Exec("TRUNCATE TABLE " + tab + ";")
		require.NoError(t, err)
	}
}

func HexDecodeString(t *testing.T, hashString string) []byte {
	t.Helper()

	hash, err := hex.DecodeString(hashString)
	require.NoError(t, err)

	return hash
}

func RevHexDecodeString(t *testing.T, hashString string) []byte {
	t.Helper()

	hash, err := hex.DecodeString(hashString)
	require.NoError(t, err)

	slices.Reverse(hash)

	return hash
}

func RevChainhash(t *testing.T, hashString string) *chainhash.Hash {
	t.Helper()

	hash, err := hex.DecodeString(hashString)
	require.NoError(t, err)
	txHash, err := chainhash.NewHash(hash)
	require.NoError(t, err)

	return txHash
}

func Chainhash(t *testing.T, hashString string) *chainhash.Hash {
	t.Helper()

	txHash, err := chainhash.NewHashFromStr(hashString)
	require.NoError(t, err)

	return txHash
}

// PtrTo returns a pointer to the given value.
func PtrTo[T any](v T) *T {
	return &v
}
