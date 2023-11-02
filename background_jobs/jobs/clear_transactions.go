package jobs

import (
	"fmt"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func init() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	logger = log.With().Str("job", "ClearTransactions").Logger()
}

func ClearTransactions(params ClearRecrodsParams) error {
	logger.Info().Msg("Connecting to database ...")
	conn, err := sqlx.Open(params.Scheme, params.String())
	if err != nil {
		logger.Err(err).Msg("unable to create connection")
		return err
	}
	interval := fmt.Sprintf("%d days", params.RecordRetentionDays)

	stmt, err := conn.Preparex("DELETE FROM transactions WHERE inserted_at <= (CURRENT_DATE - $1::interval)")
	if err != nil {
		logger.Err(err).Msg("unable to prepare statement")
		return err
	}

	res, err := stmt.Exec(interval)
	if err != nil {
		logger.Err(err).Msg("unable to delete rows")
		return err
	}
	rows, _ := res.RowsAffected()
	logger.Info().Msgf("Successfully deleted %d rows", rows)
	return nil
}
