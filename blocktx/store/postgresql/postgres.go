package postgresql

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"github.com/ordishs/gocore"
	_ "modernc.org/sqlite"
	"time"
)

const (
	postgresDriverName = "postgres"
)

type PostgreSQL struct {
	db  *sql.DB
	now func() time.Time
}

func init() {
	gocore.NewStat("blocktx")
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
		db:  db,
		now: time.Now,
	}, nil
}

func (s *PostgreSQL) Close() error {
	return s.db.Close()
}
