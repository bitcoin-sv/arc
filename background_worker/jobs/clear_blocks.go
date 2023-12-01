package jobs

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/bitcoin-sv/arc/dbconn"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type ClearRecrodsParams struct {
	dbconn.DBConnectionParams
	RecordRetentionDays int
}

func ClearBlocks(params ClearRecrodsParams) error {
	Log(INFO, "Connecting to database ...")

	conn, err := sqlx.Open(params.Scheme, params.String())
	if err != nil {
		Log(ERROR, "unable to create connection")
		return err
	}
	interval := fmt.Sprintf("%d days", params.RecordRetentionDays)

	stmt, err := conn.Preparex("DELETE FROM blocks WHERE inserted_at <= (CURRENT_DATE - $1::interval)")
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
	Log(INFO, fmt.Sprintf("Successfully deleted %d rows from blocks table", rows))

	timePassed := time.Since(start)

	logger.Info("Successfully cleared blocks table", slog.Int64("rows", rows), slog.Duration("duration", timePassed))
	return nil
}
