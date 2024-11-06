package postgresql

import (
	"context"
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
	postgresDriverName        = "postgres"
	maxPostgresBulkInsertRows = 8192
)

type QueryAble interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	Prepare(query string) (*sql.Stmt, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

type PostgreSQL struct {
	_db                       *sql.DB
	_tx                       *sql.Tx
	db                        QueryAble // this would be pointing either to _db or _tx
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
		_db:                       db,
		now:                       time.Now,
		maxPostgresBulkInsertRows: maxPostgresBulkInsertRows,
	}

	p.db = p._db

	for _, opt := range opts {
		opt(p)
	}

	return p, nil
}

func (p *PostgreSQL) BeginTx(ctx context.Context) (store.DbTransaction, error) {
	tx, err := p._db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}

	p._tx = tx
	p.db = p._tx

	return p, nil
}

func (p *PostgreSQL) Close() error {
	return p._db.Close()
}

func (p *PostgreSQL) Ping(ctx context.Context) error {
	r, err := p._db.QueryContext(ctx, "SELECT 1;")
	if err != nil {
		return err
	}

	return r.Close()
}

func (p *PostgreSQL) Commit() error {
	p.db = p._db
	return p._tx.Commit()
}

func (p *PostgreSQL) Rollback() error {
	p.db = p._db
	return p._tx.Rollback()
}

func (p *PostgreSQL) WriteLockBlocksTable(ctx context.Context) error {
	tx, ok := p.db.(*sql.Tx)
	if !ok {
		return ErrNoTransaction
	}

	// This will lock `blocks` table for writing, when performing reorg.
	// Any INSERT or UPDATE to the table will wait until the lock is released.
	// Another instance wanting to acquire this lock at the same time will have
	// to wait until the transaction holding the lock is completed and the lock
	// is released.
	//
	// Reading from the table is still allowed.
	_, err := tx.ExecContext(ctx, "LOCK TABLE blocktx.blocks IN EXCLUSIVE MODE")
	return err
}
