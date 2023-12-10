package jobs

import (
	"log/slog"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func (c ClearJob) ClearTransactions(params ClearRecrodsParams) error {
	Log(INFO, "Connecting to database ...")

	conn, err := sqlx.Open(params.Scheme(), params.String())
	if err != nil {
		Log(ERROR, "unable to create connection")
		return err
	}

	start := c.now()
	datenumRetentiondDays := start.Add(-24 * time.Hour * time.Duration(params.RecordRetentionDays))

	stmt, err := conn.Preparex("DELETE FROM transactions WHERE inserted_at_num <= $1::int")
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
	timePassed := time.Since(start)

	logger.Info("Successfully cleared transactions table", slog.Int64("rows", rows), slog.String("duration", timePassed.String()))
	return nil
}
