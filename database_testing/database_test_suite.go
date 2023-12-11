package database_testing

import (
	"errors"
	"fmt"
	mrand "math/rand"
	"path/filepath"
	"runtime"
	"time"

	"github.com/bitcoin-sv/arc/blocktx/store"
	"github.com/bitcoin-sv/arc/dbconn"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/exp/rand"
)

func GetRandomBytes() string {
	return fmt.Sprintf("%d %d", mrand.Int63(), mrand.Int63())[:32]
}
func GetTestBlock() *store.Block {
	now := time.Now()
	return &store.Block{
		ID:           int64(mrand.Intn(10000)),
		Hash:         GetRandomBytes(),
		PreviousHash: fmt.Sprintf("%d", rand.Int63()),
		MerkleRoot:   fmt.Sprintf("%d", rand.Int63()),
		Height:       int64(mrand.Intn(100)),
		Orphaned:     false,
		ProcessedAt:  now,
	}
}

func GetTestTransaction() *store.Transaction {
	return &store.Transaction{
		ID:         int64(mrand.Intn(10000)),
		Hash:       GetRandomBytes(),
		Source:     fmt.Sprintf("testtx %d", mrand.Int63()),
		MerklePath: fmt.Sprintf("testtx %d", mrand.Int63()),
	}
}

type DBConnectionParams struct {
	Host     string
	Port     int
	Username string
	Password string
	DBName   string
}

func (p DBConnectionParams) String() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", p.Username, p.Password, p.Host, p.Port, p.DBName)
}

var DefaultParams = dbconn.New(
	"localhost",
	5432,
	"arcuser",
	"arcpass",
	"blocktx_test",
	"postgres",
	"disable",
)

// DatabaseTestSuite test helper suite to
// 1. create database
// 2. run database/migrations
// 3. use in test scenario
// 4. tear down when tests are finished
type DatabaseTestSuite struct {
	suite.Suite
	Connection *sqlx.Conn
}

func (s *DatabaseTestSuite) SetupSuite() {
	_, callerFilePath, _, _ := runtime.Caller(0)

	testDir := filepath.Dir(callerFilePath)

	path := "file://" + testDir + "/../database/migrations/blocktx/postgres"
	m, err := migrate.New(path, DefaultParams.String())

	require.NoError(s.T(), err)

	if err := m.Up(); err != nil {
		if !errors.Is(err, migrate.ErrNoChange) {
			require.NoError(s.T(), err)
		}
	}
}

func (s *DatabaseTestSuite) SetupTest() {
	s.truncateTables()
}

func (s *DatabaseTestSuite) truncateTables() {
	db, err := sqlx.Open("postgres", DefaultParams.String())
	require.NoError(s.T(), err)

	db.MustExec("truncate  table blocks;")
	db.MustExec("truncate  table transactions;")
	db.MustExec("truncate  table block_transactions_map;")
}

func (s *DatabaseTestSuite) Conn() *sqlx.Conn {
	return s.Connection
}

func (s *DatabaseTestSuite) InsertBlock(block *store.Block) {
	db, err := sqlx.Open("postgres", DefaultParams.String())
	require.NoError(s.T(), err)

	q := `INSERT INTO blocks(
		id,
		hash,
		prevhash,
		merkleroot,
		orphanedyn,
		height,
		processed_at,
		inserted_at)
		VALUES(
		:id,
		:hash,
		:prevhash,
		:merkleroot,
		:orphanedyn,
		:height,
		:processed_at,
		:inserted_at
		);`

	_, err = db.NamedExec(q,
		block)
	require.NoError(s.T(), err)

}

func (s *DatabaseTestSuite) InsertTransaction(tx *store.Transaction) {
	db, err := sqlx.Open("postgres", DefaultParams.String())
	require.NoError(s.T(), err)
	q := `INSERT INTO transactions(
		id,
		hash,
		source,
		merkle_path)
		VALUES(
		:id,
		:hash,
		:source,
		:merkle_path);`

	_, err = db.NamedExec(q, tx)

	require.NoError(s.T(), err, fmt.Sprintf("tx %+v", tx))
}

func (s *DatabaseTestSuite) InsertBlockTransactionMap(btx *store.BlockTransactionMap) {
	db, err := sqlx.Open("postgres", DefaultParams.String())
	require.NoError(s.T(), err)

	q := `INSERT INTO block_transactions_map(
	blockid,
	txid,
	pos)
	VALUES(
	:blockid,
	:txid,
	:pos);`

	_, err = db.NamedExec(q, btx)
	require.NoError(s.T(), err)
}

// TearDownTest clear all the tables
func (s *DatabaseTestSuite) TearDownTest() {
	s.truncateTables()
}
