package jobs

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func ClearTransactions(params ClearRecrodsParams) error {
	Log(INFO, "Connecting to database ...")

	conn, err := sqlx.Open(params.Scheme(), params.String())
	if err != nil {
		Log(ERROR, "unable to create connection")
		return err
	}
	interval := fmt.Sprintf("%d days", params.RecordRetentionDays)

	stmt, err := conn.Preparex("DELETE FROM transactions WHERE inserted_at <= (CURRENT_DATE - $1::interval)")
	if err != nil {
		Log(ERROR, "unable to prepare statement")
		return err
	}

	start := time.Now()
	res, err := stmt.Exec(interval)
	if err != nil {
		Log(ERROR, "unable to delete rows")
		return err
	}
	rows, _ := res.RowsAffected()
	timePassed := time.Since(start)

	logger.Info("Successfully cleared transactions table", slog.Int64("rows", rows), slog.String("duration", timePassed.String()))
	return nil
}
