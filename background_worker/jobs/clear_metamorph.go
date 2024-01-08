package jobs

import (
	"fmt"

	"github.com/jmoiron/sqlx"
)

func runDelete(table string, params ClearRecordsParams) error {
	Log(INFO, "Connecting to database ...")
	conn, err := sqlx.Open(params.Scheme(), params.String())
	if err != nil {
		Log(ERROR, fmt.Sprintf("unable to create connection %s", err))
		return err
	}
	interval := fmt.Sprintf("%d days", params.RecordRetentionDays)

	stmt, err := conn.Preparex(fmt.Sprintf("DELETE FROM %s WHERE inserted_at <= (CURRENT_DATE - $1::interval)", table))
	if err != nil {
		Log(ERROR, fmt.Sprintf("unable to prepare statement %s", err))
		return err
	}

	res, err := stmt.Exec(interval)
	if err != nil {
		Log(ERROR, "unable to delete block rows")
		return err
	}
	rows, _ := res.RowsAffected()
	Log(INFO, fmt.Sprintf("Successfully deleted %d rows", rows))
	return nil
}

func ClearMetamorph(params ClearRecordsParams) error {
	if err := runDelete("metamorph.blocks", params); err != nil {
		return err
	}
	if err := runDelete("metamorph.transactions", params); err != nil {
		return err
	}

	return nil
}
