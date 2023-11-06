package jobs

import (
	"fmt"
	logger "log/slog"

	"github.com/bitcoin-sv/arc/dbconn"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

const (
	INFO  = "INFO"
	DEBUG = "DEBUG"
	ERROR = "ERROR"
)

func Log(level, message string) {
	var logfun = map[string]func(string, ...any){
		INFO:  logger.Info,
		DEBUG: logger.Debug,
		ERROR: logger.Error,
	}
	logfun[level](message)
}

type ClearRecrodsParams struct {
	dbconn.DBConnectionParams
	RecordRetentionDays int //delete blocks that are RecordRetentionDays days old
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

	res, err := stmt.Exec(interval)
	if err != nil {
		Log(ERROR, "unable to delete rows")
		return err
	}
	rows, _ := res.RowsAffected()
	Log(INFO, fmt.Sprintf("Successfully deleted %d rows", rows))
	return nil
}
