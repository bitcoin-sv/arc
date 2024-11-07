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
	dbInfo                    string
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
		dbInfo:                    dbInfo,
	}

	p.db = p._db

	for _, opt := range opts {
		opt(p)
	}

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

func (p *PostgreSQL) StartUnitOfWork(ctx context.Context) (store.UnitOfWork, error) {
	// This will create a clone of the store and start a transaction
	// to avoid messing with the state of the main singleton store
	cloneDB, err := sql.Open(postgresDriverName, p.dbInfo)
	if err != nil {
		return nil, errors.Join(store.ErrFailedToOpenDB, err)
	}

	cloneStore := &PostgreSQL{
		_db:                       cloneDB,
		now:                       time.Now,
		maxPostgresBulkInsertRows: maxPostgresBulkInsertRows,
		tracingEnabled:            p.tracingEnabled,
		tracingAttributes:         p.tracingAttributes,
	}

	tx, err := cloneStore._db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}

	cloneStore._tx = tx
	cloneStore.db = cloneStore._tx

	return cloneStore, nil
}

// UnitOfWork methods below
func (uow *PostgreSQL) Commit() error {
	uow.db = uow._db
	return uow._tx.Commit()
}

func (uow *PostgreSQL) Rollback() error {
	uow.db = uow._db
	return uow._tx.Rollback()
}

func (uow *PostgreSQL) WriteLockBlocksTable(ctx context.Context) error {
	tx, ok := uow.db.(*sql.Tx)
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
