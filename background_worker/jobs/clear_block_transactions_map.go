package jobs

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func (c ClearJob) ClearBlockTransactionsMap(params ClearRecordsParams) error {
	Log(INFO, "Connecting to database ...")
	conn, err := sqlx.Open(params.Scheme(), params.String())

	if err != nil {
		Log(ERROR, fmt.Sprintf("unable to create connection %s", err))
		return err
	}

	start := c.now()
	deleteBeforeDate := start.Add(-24 * time.Hour * time.Duration(params.RecordRetentionDays))

	stmt, err := conn.Preparex("DELETE FROM block_transactions_map WHERE inserted_at_num <= $1::int")
	if err != nil {
		Log(ERROR, fmt.Sprintf("unable to prepare statement %s", err))
		return err
	}

	res, err := stmt.Exec(deleteBeforeDate.Format(numericalDateHourLayout))
	if err != nil {
		Log(ERROR, "unable to delete rows")
		return err
	}
	rows, _ := res.RowsAffected()

	timePassed := time.Since(start)

	logger.Info("Successfully cleared block_transactions_map table", slog.Int64("rows", rows), slog.String("duration", timePassed.String()))
	return nil
}
