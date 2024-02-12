package postgresql

import (
	"database/sql"
	"fmt"
	"github.com/bitcoin-sv/arc/blocktx/store"
	"github.com/bitcoin-sv/arc/dbconn"
	_ "github.com/lib/pq"
	"github.com/ordishs/gocore"
	_ "modernc.org/sqlite"
)

const (
	postgresDriverName = "postgres"
)

type PostgreSQL struct {
	db *sql.DB
}

func init() {
	gocore.NewStat("blocktx")
}

// NewPostgresStore postgres storage that accepts connection parameters and returns connection or an error.
func NewPostgresStore(params dbconn.DBConnectionParams) (store.Interface, error) {
	db, err := sql.Open("postgres", params.String())
	if err != nil {
		return nil, fmt.Errorf("unable to connect to db due to %s", err)
	}

	return &PostgreSQL{
		db: db,
	}, nil
}

func New(dbInfo string, idleConns int, maxOpenConns int) (*PostgreSQL, error) {
	var db *sql.DB
	var err error

	db, err = sql.Open(postgresDriverName, dbInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres DB: %+v", err)
	}

	db.SetMaxIdleConns(idleConns)
	db.SetMaxOpenConns(maxOpenConns)

	return &PostgreSQL{
		db: db,
	}, nil
}

func (s *PostgreSQL) Close() error {
	return s.db.Close()
}
