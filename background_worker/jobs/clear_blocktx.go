package jobs

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/bitcoin-sv/arc/dbconn"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

const (
	numericalDateHourLayout = "2006010215"
)

type ClearRecordsParams struct {
	dbconn.DBConnectionParams
	RecordRetentionDays int
}

type ClearJob struct {
	now    func() time.Time
	logger *slog.Logger
}

func WithNow(nowFunc func() time.Time) func(*ClearJob) {
	return func(p *ClearJob) {
		p.now = nowFunc
	}
}

func NewClearJob(logger *slog.Logger, opts ...func(job *ClearJob)) *ClearJob {
	c := &ClearJob{
		now:    time.Now,
		logger: logger,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c ClearJob) ClearBlocktxTable(params ClearRecordsParams, table string) error {
	c.logger.Info("Connecting to database")

	conn, err := sqlx.Open(params.Scheme(), params.String())
	if err != nil {
		return fmt.Errorf("unable to create connection: %v", err)
	}

	start := c.now()
	deleteBeforeDate := start.Add(-24 * time.Hour * time.Duration(params.RecordRetentionDays))

	stmt, err := conn.Preparex(fmt.Sprintf("DELETE FROM %s WHERE inserted_at_num <= $1::int", table))
	if err != nil {
		return fmt.Errorf("unable to prepare statement: %v", err)
	}

	res, err := stmt.Exec(deleteBeforeDate.Format(numericalDateHourLayout))
	if err != nil {
		return fmt.Errorf("unable to delete rows: %v", err)
	}
	rows, _ := res.RowsAffected()
	timePassed := time.Since(start)

	c.logger.Info("cleared blocktx data", slog.String("table", table), slog.Int64("rows", rows), slog.String("duration", timePassed.String()))
	return nil
}
