package sql

import (
	"crypto/rand"
	"database/sql"
	mrand "math/rand"
	"testing"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/golang-migrate/migrate/v4"
	"github.com/stretchr/testify/require"
)

func getRandomBytes() []byte {
	hash := make([]byte, 32)
	rand.Read(hash)
	return hash
}
func GetTestBlock() *blocktx_api.Block {
	return &blocktx_api.Block{
		Hash:         getRandomBytes(),
		PreviousHash: getRandomBytes(),
		MerkleRoot:   getRandomBytes(),
		Height:       mrand.Uint64(),
		Orphaned:     true,
	}
}

func connect() (*sql.DB, error) {
	db, err := sql.Open("postgres", "host=localhost port=5432 user=postgres password=postgres dbname=postgres sslmode=disable")
	return db, err
}

func TestGetBlock(t *testing.T) {
	embeddedpostgres := embeddedpostgres.NewDatabase()

	err := embeddedpostgres.Start()

	require.NoError(t, err)

	db, err := connect()

	require.NoError(t, err)

	migrate.NewWithDatabaseInstance(db, "postgres")
	//b := GetTestBlock()
	//
	//store, err := NewStore(DBConnectionParams{})
	//
	//require.NoError(t, err)
	//
	//store.GetBlock()

}
