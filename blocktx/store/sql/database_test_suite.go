package sql

import (
	"fmt"
	mrand "math/rand"
	"path/filepath"
	"runtime"
	"time"

	"github.com/bitcoin-sv/arc/blocktx/store"
	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/exp/rand"
)

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

var defaultParams = DBConnectionParams{
	Host:     "localhost",
	Port:     5432,
	Username: "postgres",
	Password: "postgres",
	DBName:   "postgres",
}

// DatabaseTestSuite test helper suite to
// 1. create database
// 2. run database/migrations
// 3. use in test scenario
// 4. tear down when tests are finished
type DatabaseTestSuite struct {
	suite.Suite
	Database *embeddedpostgres.EmbeddedPostgres
}

func (s *DatabaseTestSuite) TearDown() {
	s.TearDownSuite()
}

func (s *DatabaseTestSuite) SetupSuite() {
	s.Database = embeddedpostgres.NewDatabase()

	require.NoError(s.T(), s.Database.Start())

	_, callerFilePath, _, _ := runtime.Caller(0)

	testDir := filepath.Dir(callerFilePath)

	path := "file://" + testDir + "/../../../database/migrations/postgres"
	m, err := migrate.New(path, defaultParams.String())
	require.NoError(s.T(), err)
	require.NoError(s.T(), m.Up())
}

func getRandomBytes() string {
	return fmt.Sprintf("%d %d", mrand.Int63(), mrand.Int63())[:32]
}

func GetTestBlock() *store.Block {
	now := time.Now()
	return &store.Block{
		ID:           mrand.Intn(100),
		Hash:         getRandomBytes(),
		PreviousHash: fmt.Sprintf("%d", rand.Int63()),
		MerkleRoot:   fmt.Sprintf("%d", rand.Int63()),
		Height:       mrand.Intn(100),
		Orphaned:     false,
		ProcessedAt:  &now,
	}
}

func GetTestTransaction() *store.Transaction {
	return &store.Transaction{
		ID:         mrand.Int(),
		Hash:       getRandomBytes(),
		Source:     fmt.Sprintf("testtx %d", mrand.Int63()),
		MerklePath: fmt.Sprintf("testtx %d", mrand.Int63()),
	}
}

func (s *DatabaseTestSuite) InsertBlock(block *store.Block) {
	db, err := sqlx.Open("postgres", defaultParams.String())
	require.NoError(s.T(), err)

	_, err = db.NamedExec("INSERT INTO blocks("+
		"id, "+
		"hash, "+
		"prevhash, "+
		"merkleroot, "+
		"orphanedyn, "+
		"height,"+
		"processed_at) "+
		"VALUES("+
		":id,"+
		":hash, "+
		":prevhash, "+
		":merkleroot, "+
		":orphanedyn, "+
		":height,"+
		":processed_at);", block)
	require.NoError(s.T(), err)

}

func (s *DatabaseTestSuite) InsertTransaction(tx *store.Transaction) {
	db, err := sqlx.Open("postgres", defaultParams.String())
	require.NoError(s.T(), err)

	_, err = db.NamedExec("INSERT INTO transactions("+
		"hash, "+
		"source, "+
		"merkle_path) "+
		"VALUES("+
		":hash, "+
		":source, "+
		":merkle_path); ", tx)

	require.NoError(s.T(), err, fmt.Sprintf("tx %+v", tx))
}

func (s *DatabaseTestSuite) InsertBlockTransactionMap(btx *store.BlockTransactionMap) {
	db, err := sqlx.Open("postgres", defaultParams.String())
	require.NoError(s.T(), err)

	_, err = db.NamedExec("INSERT INTO block_transactions_map("+
		"blockid, "+
		"txid, "+
		"pos) "+
		"VALUES("+
		":blockid,"+
		":txid, "+
		":pos);", btx)
	require.NoError(s.T(), err)
}

func (s *DatabaseTestSuite) TearDownSuite() {
	require.NoError(s.T(), s.Database.Stop())
}

// TearDownTest clear all the tables
func (s *DatabaseTestSuite) TearDownTest() {
	db, err := sqlx.Open("postgres", defaultParams.String())
	require.NoError(s.T(), err)

	db.MustExec("truncate  table blocks;")
	db.MustExec("truncate  table transactions;")
	db.MustExec("truncate  table block_transactions_map;")
}
