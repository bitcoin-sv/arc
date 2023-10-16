package sql

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	mrand "math/rand"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/bitcoin-sv/arc/blocktx/store"
	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

var conn_url = fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", "postgres", "postgres", "localhost", 5432, "postgres")

func getRandomBytes() []byte {
	hash := make([]byte, chainhash.HashSize)
	_, err := rand.Read(hash)
	if err != nil {
		log.Fatal(err)
	}
	return hash
}
func GetTestBlock() *store.Block {
	return &store.Block{
		ID:           mrand.Int63(),
		Hash:         getRandomBytes(),
		PreviousHash: getRandomBytes(),
		MerkleRoot:   getRandomBytes(),
		Height:       mrand.Int63(),
		Orphaned:     true,
	}
}

type DatabaseTestSuite struct {
	suite.Suite

	Database *embeddedpostgres.EmbeddedPostgres
}

func (s *DatabaseTestSuite) SetupSuite() {
	s.Database = embeddedpostgres.NewDatabase()

	require.NoError(s.T(), s.Database.Start())

	_, callerFilePath, _, _ := runtime.Caller(0)

	// Calculate the directory path of the test file
	testDir := filepath.Dir(callerFilePath)

	url := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", "postgres", "postgres", "localhost", 5432, "postgres")
	path := "file://" + testDir + "/../../../database/migrations/"
	m, err := migrate.New(path, url)
	require.NoError(s.T(), err)

	require.NoError(s.T(), m.Up())

}

func (s *DatabaseTestSuite) TearDownSuite() {
	require.NoError(s.T(), s.Database.Stop())
}

func (s *DatabaseTestSuite) TestGetBlock() {
	db, err := sqlx.Open("postgres", conn_url)
	require.NoError(s.T(), err)

	block := GetTestBlock()

	_, err = db.NamedExec("INSERT INTO blocks("+
		"id, "+
		"hash, "+
		"prevhash, "+
		"merkleroot, "+
		"height) "+
		"VALUES("+
		":id,"+
		":hash, "+
		":prevhash, "+
		":merkleroot, "+
		":height);", block)
	require.NoError(s.T(), err)

	store, err := NewStore(DBConnectionParams{
		Host:     "localhost",
		Port:     5432,
		Username: "postgres",
		Password: "postgres",
		DBName:   "postgres",
		Engine:   "postgres",
	})

	require.NoError(s.T(), err)

	h, err := chainhash.NewHash(block.Hash)
	require.NoError(s.T(), err)
	b, err := store.GetBlock(context.Background(), h)

	require.NoError(s.T(), err)

	assert.Equal(s.T(), block.Hash, b.Hash)

}

func TestRunDatabaseTestSuite(t *testing.T) {
	s := new(DatabaseTestSuite)
	suite.Run(t, s)
	if err := recover(); err != nil {
		require.NoError(t, s.Database.Stop())
	}
}
