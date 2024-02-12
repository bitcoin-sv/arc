package postgresql

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/blocktx/store"
	"github.com/bitcoin-sv/arc/testdata"
	"github.com/go-testfixtures/testfixtures/v3"
	"github.com/golang-migrate/migrate/v4"
	migratepostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/require"
)

type Block struct {
	ID           int64     `db:"id"`
	Hash         string    `db:"hash"`
	PreviousHash string    `db:"prevhash"`
	MerkleRoot   string    `db:"merkleroot"`
	MerklePath   *string   `db:"merkle_path"`
	Height       int64     `db:"height"`
	Orphaned     bool      `db:"orphanedyn"`
	Size         *int64    `db:"size"`
	TxCount      *int64    `db:"tx_count"`
	Processed    bool      `db:"processed"`
	ProcessedAt  time.Time `db:"processed_at"`
	InsertedAt   time.Time `db:"inserted_at"`
}

type Transaction struct {
	ID         int64  `db:"id"`
	Hash       []byte `db:"hash"`
	MerklePath string `db:"merkle_path"`
}

type BlockTransactionMap struct {
	BlockID       int64 `db:"blockid"`
	TransactionID int64 `db:"txid"`
	Pos           int64 `db:"pos"`
}

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
	fmt.Println(dbInfo)
	var postgresDB *PostgreSQL
	err = pool.Retry(func() error {
		postgresDB, err = New(dbInfo, 10, 10)
		if err != nil {
			return err
		}
		return postgresDB.db.Ping()
	})
	if err != nil {
		log.Fatalf("failed to connect to docker: %s", err)
	}

	driver, err := migratepostgres.WithInstance(postgresDB.db, &migratepostgres.Config{
		MigrationsTable: "blocktx",
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

	blockHash2, err := chainhash.NewHashFromStr("00000000000000000a081a539601645abe977946f8f6466a3c9e0c34d50be4a8")
	require.NoError(t, err)

	blockHash1, err := chainhash.NewHashFromStr("000000000000000001b8adefc1eb98896c80e30e517b9e2655f1f929d9958a48")
	require.NoError(t, err)

	merkleRoot, err := chainhash.NewHashFromStr("31e25c5ac7c143687f55fc49caf0f552ba6a16d4f785e4c9a9a842179a085f0c")
	require.NoError(t, err)

	postgresDB, err := New(dbInfo, 10, 10, WithNow(func() time.Time {
		return time.Date(2023, 12, 22, 12, 0, 0, 0, time.UTC)
	}))
	ctx := context.Background()
	require.NoError(t, err)
	defer func() {
		postgresDB.Close()
	}()
	t.Run("insert block / get block", func(t *testing.T) {
		block := &blocktx_api.Block{
			Hash:         blockHash2[:],
			PreviousHash: blockHash1[:],
			MerkleRoot:   merkleRoot[:],
			Height:       100,
		}

		id, err := postgresDB.InsertBlock(ctx, block)
		require.NoError(t, err)
		fmt.Println(id)
		require.Equal(t, uint64(10001), id)

		blockResp, err := postgresDB.GetBlock(ctx, blockHash2)
		require.NoError(t, err)
		require.Equal(t, block, blockResp)
	})

	t.Run("register transactions", func(t *testing.T) {
		txs := []*blocktx_api.TransactionAndSource{
			{
				Hash: testdata.TX1Hash[:],
			},
			{
				Hash: testdata.TX2Hash[:],
			},
			{
				Hash: testdata.TX3Hash[:],
			},
			{
				Hash: testdata.TX4Hash[:],
			},
		}
		err = postgresDB.RegisterTransactions(context.Background(), txs)
		require.NoError(t, err)

		d, err := sqlx.Open("postgres", dbInfo)
		require.NoError(t, err)

		var storedtx Transaction
		err = d.Get(&storedtx, "SELECT id, hash, merkle_path from transactions WHERE hash=$1", string(txs[0].GetHash()))
		require.NoError(t, err)

		require.True(t, bytes.Equal(txs[0].GetHash(), storedtx.Hash))

		err = d.Get(&storedtx, "SELECT id, hash, merkle_path from transactions WHERE hash=$1", string(txs[1].GetHash()))
		require.NoError(t, err)

		require.True(t, bytes.Equal(txs[1].GetHash(), storedtx.Hash))

		err = d.Get(&storedtx, "SELECT id, hash, merkle_path from transactions WHERE hash=$1", string(txs[2].GetHash()))
		require.NoError(t, err)

		require.True(t, bytes.Equal(txs[2].GetHash(), storedtx.Hash))

		err = d.Get(&storedtx, "SELECT id, hash, merkle_path from transactions WHERE hash=$1", string(txs[3].GetHash()))
		require.NoError(t, err)

		require.True(t, bytes.Equal(txs[3].GetHash(), storedtx.Hash))

		newHash1, err := hex.DecodeString("bb6817dad4c9a0e34803c2f1dec13ac8b89d539a57749be808d61f314bbbd855")
		require.NoError(t, err)

		newHash2, err := hex.DecodeString("ec52c6f777e6cb254d596001050f4151ef74cea78b6d50163407bf54bdd54e3f")
		require.NoError(t, err)
		// Register transactions which are partly already registered
		txs2 := []*blocktx_api.TransactionAndSource{
			txs[0],
			txs[2],
			{
				Hash: newHash1,
			},
			{
				Hash: newHash2,
			},
		}
		err = postgresDB.RegisterTransactions(context.Background(), txs2)
		require.NoError(t, err)

		err = d.Get(&storedtx, "SELECT id, hash, merkle_path from transactions WHERE hash=$1", string(txs2[2].GetHash()))
		require.NoError(t, err)

		require.True(t, bytes.Equal(txs2[2].GetHash(), storedtx.Hash))

		err = d.Get(&storedtx, "SELECT id, hash, merkle_path from transactions WHERE hash=$1", string(txs2[3].GetHash()))
		require.NoError(t, err)

		require.True(t, bytes.Equal(txs2[3].GetHash(), storedtx.Hash))
	})

	t.Run("get block gaps", func(t *testing.T) {
		blockGaps, err := postgresDB.GetBlockGaps(ctx, 7)
		require.NoError(t, err)
		require.Equal(t, 2, len(blockGaps))

		hash822014, err := chainhash.NewHashFromStr("0000000000000000025855b62f4c2e3732dad363a6f2ead94e4657ef96877067")
		require.NoError(t, err)
		hash822019, err := chainhash.NewHashFromStr("00000000000000000364332e1bbd61dc928141b9469c5daea26a4b506efc9656")
		require.NoError(t, err)

		expectedBlockGaps := []*store.BlockGap{
			{
				Height: 822019,
				Hash:   hash822019,
			},
			{
				Height: 822014,
				Hash:   hash822014,
			},
		}

		require.ElementsMatch(t, expectedBlockGaps, blockGaps)
	})

	t.Run("update block transactions", func(t *testing.T) {
		testMerklePaths := []string{"test1", "test2", "test3"}
		testBlockID := uint64(9736)

		txHash1, err := chainhash.NewHashFromStr("3b8c1676470f44043069f66fdc0d5df9bdad1865a8f03b8da1268359802b7376")
		require.NoError(t, err)

		txHash2, err := chainhash.NewHashFromStr("b0926372c449731ffb84b7d2f808087d4b5e6e26aafee872232bbcd5a5854e16")
		require.NoError(t, err)

		txHashNotRegistered, err := chainhash.NewHashFromStr("edd33fdcdfa68444d227780e2b62a4437c00120c5320d2026aeb24a781f4c3f1")
		require.NoError(t, err)

		_, err = postgresDB.UpdateBlockTransactions(context.Background(), testBlockID, []*blocktx_api.TransactionAndSource{
			{
				Hash: txHash1[:],
			},
			{
				Hash: txHash2[:],
			},
			{
				Hash: txHashNotRegistered[:],
			},
		}, []string{"test1"})

		require.ErrorContains(t, err, "transactions (len=3) and Merkle paths (len=1) have not the same lengths")

		_, err = postgresDB.UpdateBlockTransactions(context.Background(), testBlockID, []*blocktx_api.TransactionAndSource{
			{
				Hash: txHash1[:],
			},
			{
				Hash: txHash2[:],
			},
			{
				Hash: txHashNotRegistered[:],
			},
		}, testMerklePaths)
		require.NoError(t, err)

		d, err := sqlx.Open("postgres", dbInfo)
		require.NoError(t, err)

		var storedtx Transaction

		err = d.Get(&storedtx, "SELECT id, hash, merkle_path from transactions WHERE hash=$1", txHash1[:])
		require.NoError(t, err)

		require.Equal(t, txHash1[:], storedtx.Hash)
		require.Equal(t, testMerklePaths[0], storedtx.MerklePath)

		var mp BlockTransactionMap
		err = d.Get(&mp, "SELECT blockid, txid, pos from block_transactions_map WHERE txid=$1", storedtx.ID)
		require.NoError(t, err)

		require.Equal(t, storedtx.ID, mp.TransactionID)
		require.Equal(t, testBlockID, uint64(mp.BlockID))

		var storedtx2 Transaction

		err = d.Get(&storedtx2, "SELECT id, hash, merkle_path from transactions WHERE hash=$1", txHash2[:])
		require.NoError(t, err)

		require.Equal(t, txHash2[:], storedtx2.Hash)
		require.Equal(t, testMerklePaths[1], storedtx2.MerklePath)

		var mp2 BlockTransactionMap
		err = d.Get(&mp2, "SELECT blockid, txid, pos from block_transactions_map WHERE txid=$1", storedtx2.ID)
		require.NoError(t, err)

		require.Equal(t, storedtx2.ID, mp2.TransactionID)
		require.Equal(t, testBlockID, uint64(mp2.BlockID))

	})

	t.Run("clear data", func(t *testing.T) {

		resp, err := postgresDB.ClearBlocktxTable(context.Background(), 10, "blocks")
		require.NoError(t, err)
		require.Equal(t, int64(1), resp.Rows)

		d, err := sqlx.Open("postgres", dbInfo)
		require.NoError(t, err)

		var blocks []Block

		require.NoError(t, d.Select(&blocks, "SELECT id FROM blocks"))

		require.Len(t, blocks, 1)

		resp, err = postgresDB.ClearBlocktxTable(context.Background(), 10, "block_transactions_map")
		require.NoError(t, err)
		require.Equal(t, int64(5), resp.Rows)

		var mps []BlockTransactionMap
		require.NoError(t, d.Select(&mps, "SELECT blockid FROM block_transactions_map"))

		require.Len(t, mps, 5)

		resp, err = postgresDB.ClearBlocktxTable(context.Background(), 10, "transactions")
		require.NoError(t, err)
		require.Equal(t, int64(5), resp.Rows)

		var txs []Transaction
		require.NoError(t, d.Select(&txs, "SELECT id FROM transactions"))

		require.Len(t, txs, 5)
	})
}
