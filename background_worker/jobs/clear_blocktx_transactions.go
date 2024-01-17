package jobs

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func (c ClearJob) ClearTransactions(params ClearRecordsParams) error {
	c.logger.Info("Connecting to database")

	conn, err := sqlx.Open(params.Scheme(), params.String())
	if err != nil {
		return fmt.Errorf("unable to create connection: %v", err)
	}

	start := c.now()
	deleteBeforeDate := start.Add(-24 * time.Hour * time.Duration(params.RecordRetentionDays))

	stmt, err := conn.Preparex("DELETE FROM transactions WHERE inserted_at_num <= $1::int")
	if err != nil {
		return fmt.Errorf("unable to prepare statement: %v", err)
	}

	res, err := stmt.Exec(deleteBeforeDate.Format(numericalDateHourLayout))
	if err != nil {
		return fmt.Errorf("unable to delete rows: %v", err)
	}
	rows, _ := res.RowsAffected()
	timePassed := time.Since(start)

	c.logger.Info("cleared transactions table", slog.Int64("rows", rows), slog.String("duration", timePassed.String()))
	return nil
}
