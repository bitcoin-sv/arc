package jobs

import (
	"fmt"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func ClearBlockTransactionsMap(params ClearRecrodsParams) error {
	Log(INFO, "Connecting to database ...")
	conn, err := sqlx.Open(params.Scheme, params.String())
	if err != nil {
		Log(ERROR, fmt.Sprintf("unable to create connection %s", err))
		return err
	}
	interval := fmt.Sprintf("%d days", params.RecordRetentionDays)

	stmt, err := conn.Preparex("DELETE FROM block_transactions_map WHERE inserted_at <= (CURRENT_DATE - $1::interval)")
	if err != nil {
		Log(ERROR, fmt.Sprintf("unable to prepare statement %s", err))
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
