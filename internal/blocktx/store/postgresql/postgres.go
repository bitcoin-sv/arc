package postgresql

import (
	"context"
	"database/sql"
	"errors"
	"time"

	_ "github.com/lib/pq" // nolint: revive // required for postgres driver
	"go.opentelemetry.io/otel/attribute"

	"github.com/bitcoin-sv/arc/internal/blocktx/store"
)

const (
	postgresDriverName        = "postgres"
	maxPostgresBulkInsertRows = 8192
)

type PostgreSQL struct {
	db                        *sql.DB
	now                       func() time.Time
	maxPostgresBulkInsertRows int
	tracingEnabled            bool
	attributes                []attribute.KeyValue
}

func WithNow(nowFunc func() time.Time) func(*PostgreSQL) {
	return func(p *PostgreSQL) {
		p.now = nowFunc
	}
}

func WithTracer(attr []attribute.KeyValue) func(handler *PostgreSQL) {
	return func(p *PostgreSQL) {
		p.tracingEnabled = true
		p.attributes = attr
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

func (p *PostgreSQL) Ping(ctx context.Context) error {
	r, err := p.db.QueryContext(ctx, "SELECT 1;")
	if err != nil {
		return err
	}

	return r.Close()
}
