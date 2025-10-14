package postgresql

import (
	"database/sql"
	"errors"
	"runtime"
	"time"

	_ "github.com/lib/pq" // nolint: revive // required for postgres driver
	"go.opentelemetry.io/otel/attribute"

	"github.com/bitcoin-sv/arc/internal/blocktx/store"
)

var ErrNoTransaction = errors.New("sql: transaction has already been committed or rolled back")

const (
	postgresDriverName        = "pgx"
	maxPostgresBulkInsertRows = 8192
)

type PostgreSQL struct {
	db                        *sql.DB
	now                       func() time.Time
	maxPostgresBulkInsertRows int
	tracingEnabled            bool
	tracingAttributes         []attribute.KeyValue
}

func WithNow(nowFunc func() time.Time) func(*PostgreSQL) {
	return func(p *PostgreSQL) {
		p.now = nowFunc
	}
}

func WithTracer(attr ...attribute.KeyValue) func(s *PostgreSQL) {
	return func(p *PostgreSQL) {
		p.tracingEnabled = true
		if len(attr) > 0 {
			p.tracingAttributes = append(p.tracingAttributes, attr...)
		}
		_, file, _, ok := runtime.Caller(1)
		if ok {
			p.tracingAttributes = append(p.tracingAttributes, attribute.String("file", file))
		}
	}
}

func New(dbInfo string, idleConns int, maxOpenConns int, opts ...func(postgreSQL *PostgreSQL)) (*PostgreSQL, error) {
	var db *sql.DB
	var err error
	db, err = sql.Open(postgresDriverName, dbInfo)
	if err != nil {
		return nil, errors.Join(store.ErrFailedToOpenDB, err)
	}
	db.SetMaxIdleConns(idleConns)
	db.SetMaxOpenConns(maxOpenConns)

	p := &PostgreSQL{
		db:                        db,
		now:                       time.Now,
		maxPostgresBulkInsertRows: maxPostgresBulkInsertRows,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p, nil
}
func (p *PostgreSQL) Close() error {
	return p.db.Close()
}

func (p *PostgreSQL) Ping() error {
	return p.db.Ping()
}
