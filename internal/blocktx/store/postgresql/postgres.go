package postgresql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

const (
	postgresDriverName        = "postgres"
	maxPostgresBulkInsertRows = 8192
)

var tracer trace.Tracer

type PostgreSQL struct {
	db                        *sql.DB
	now                       func() time.Time
	maxPostgresBulkInsertRows int
}

func WithNow(nowFunc func() time.Time) func(*PostgreSQL) {
	return func(p *PostgreSQL) {
		p.now = nowFunc
	}
}

func WithTracer() func(handler *PostgreSQL) {
	return func(_ *PostgreSQL) {
		tracer = otel.GetTracerProvider().Tracer("")
	}
}

func New(dbInfo string, idleConns int, maxOpenConns int, opts ...func(postgreSQL *PostgreSQL)) (*PostgreSQL, error) {
	var db *sql.DB
	var err error

	db, err = sql.Open(postgresDriverName, dbInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres DB: %+v", err)
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
