package database_testing

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"time"

	"github.com/bitcoin-sv/arc/dbconn"
	"github.com/bitcoin-sv/arc/metamorph/store"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func GetTestMMBlock() *store.Block {
	return &store.Block{
		Hash:        GetRandomBytes(),
		ProcessedAt: time.Now().UTC(),
		InsertedAt:  time.Now().UTC(),
	}
}

func GetTestMMTransaction() *store.Transaction {
	return &store.Transaction{
		Hash:          GetRandomBytes(),
		StoredAt:      time.Date(2023, 10, 4, 22, 0, 0, 0, time.UTC),
		AnnouncedAt:   time.Date(2023, 10, 5, 12, 0, 0, 0, time.UTC),
		MinedAt:       time.Date(2023, 10, 7, 10, 0, 0, 0, time.UTC),
		Status:        1,
		BlockHeight:   int64(12345),
		BlockHash:     []byte{0x11, 0x22, 0x33, 0x44},
		CallbackURL:   "https://example.com/callback",
		CallbackToken: "1234567890abcdef",
		MerkleProof:   "4d1f934bd7a5223b95656220d39c64b3a66b0f770137f6611052381806e90275",
		RawTX:         []byte("01020304"), // example raw transaction data
		LockedBy:      "0x1234567890abcdef",
		InsertedAt:    time.Now().UTC(),
	}
}

var DefaultMMParams = dbconn.New(
	"localhost",
	5432,
	"arcuser",
	"arcpass",
	"metamorph_test",
	"postgres",
	"disable",
)

type MetamorphDBTestSuite struct {
	suite.Suite
	Connection *sqlx.Conn
}

func (s *MetamorphDBTestSuite) SetupSuite() {
	_, callerFilePath, _, _ := runtime.Caller(0)

	testDir := filepath.Dir(callerFilePath)

	path := "file://" + testDir + "/../database/migrations/metamorph/postgres"
	m, err := migrate.New(path, DefaultMMParams.String())
	require.NoError(s.T(), err)
	s.T().Log("applied migrations")
	if err := m.Up(); err != nil {
		if !errors.Is(err, migrate.ErrNoChange) {
			require.NoError(s.T(), err)
		}
	}
}

func (s *MetamorphDBTestSuite) SetupTest() {
	s.truncateTables()
}

func (s *MetamorphDBTestSuite) truncateTables() {
	s.T().Log("truncating tables")
	db, err := sqlx.Open("postgres", DefaultMMParams.String())
	require.NoError(s.T(), err)

	db.MustExec("truncate  table metamorph.blocks;")
	db.MustExec("truncate  table metamorph.transactions;")
}

func (s *MetamorphDBTestSuite) Conn() *sqlx.Conn {
	return s.Connection
}

func (s *MetamorphDBTestSuite) InsertBlock(block *store.Block) {
	db, err := sqlx.Open("postgres", DefaultMMParams.String())
	require.NoError(s.T(), err)

	q := `INSERT INTO metamorph.blocks(
		hash,
		processed_at,
		inserted_at)
		VALUES(
		:hash,
		:processed_at,
		:inserted_at
		);`

	_, err = db.NamedExec(q,
		block)
	require.NoError(s.T(), err)

}

func (s *MetamorphDBTestSuite) InsertTransaction(tx *store.Transaction) {
	db, err := sqlx.Open("postgres", DefaultMMParams.String())
	require.NoError(s.T(), err)
	q := `INSERT INTO metamorph.transactions (hash, 
     stored_at, 
     announced_at, 
     mined_at, 
     status, 
     block_height, 
     block_hash, 
     callback_url, 
     callback_token, 
     merkle_proof, 
     reject_reason, 
     raw_tx, 
     locked_by, 
     inserted_at) VALUES (
     :hash, 
     :stored_at, 
     :announced_at, 
     :mined_at, 
     :status, 
     :block_height, 
     :block_hash, 
     :callback_url, 
     :callback_token, 
     :merkle_proof, 
     :reject_reason, 
     :raw_tx, 
     :locked_by, 
     :inserted_at);`

	_, err = db.NamedExec(q, tx)

	require.NoError(s.T(), err, fmt.Sprintf("tx %+v", tx))
}

// TearDownTest clear all the tables
func (s *MetamorphDBTestSuite) TearDownTest() {
	s.truncateTables()
}
