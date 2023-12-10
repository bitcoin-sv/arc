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
	datenumHourlyParsing = "2006010215"
)

type ClearRecrodsParams struct {
	dbconn.DBConnectionParams
	RecordRetentionDays int
}

type ClearJob struct {
	now func() time.Time
}

func WithNow(nowFunc func() time.Time) func(*ClearJob) {
	return func(p *ClearJob) {
		p.now = nowFunc
	}
}

func NewClearJob(opts ...func(job *ClearJob)) *ClearJob {
	c := &ClearJob{
		now: time.Now,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c ClearJob) ClearBlocks(params ClearRecrodsParams) error {
	Log(INFO, "Connecting to database ...")

	conn, err := sqlx.Open(params.Scheme(), params.String())
	if err != nil {
		Log(ERROR, "unable to create connection")
		return err
	}

	start := c.now()
	datenumRetentiondDays := start.Add(-24 * time.Hour * time.Duration(params.RecordRetentionDays))

	stmt, err := conn.Preparex("DELETE FROM blocks WHERE inserted_at_num <= $1::int")
	if err != nil {
		Log(ERROR, "unable to prepare statement")
		return err
	}

	res, err := stmt.Exec(datenumRetentiondDays.Format(datenumHourlyParsing))
	if err != nil {
		Log(ERROR, "unable to delete rows")
		return err
	}
	rows, _ := res.RowsAffected()
	Log(INFO, fmt.Sprintf("Successfully deleted %d rows from blocks table", rows))

	timePassed := time.Since(start)

	logger.Info("Successfully cleared blocks table", slog.Int64("rows", rows), slog.String("duration", timePassed.String()))
	return nil
}
