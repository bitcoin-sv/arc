package jobs

import (
	"github.com/bitcoin-sv/arc/dbconn"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var logger zerolog.Logger

func init() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	logger = log.With().Str("job", "ClearBlocks").Logger()
}

type Params struct {
	dbconn.DBConnectionParams
	OlderThan int //delete blocks that are OlderThan days old
}

func ClearBlocks(params Params) error {
	logger.Info().Msg("Connecting to database ...")
	conn, err := sqlx.Open(params.Scheme, params.String())
	if err != nil {
		logger.Err(err).Msg("unable to create connection")
		return err
	}
	stmt, err := conn.Prepare("DELETE FROM blocks WHERE inserted_at <= (CURRENT_DATE - INTERVAL '$1 days')")
	if err != nil {
		logger.Err(err).Msg("unable to prepare statement")
		return err
	}

	res, err := stmt.Exec(params.OlderThan)
	if err != nil {
		logger.Err(err).Msg("unable to delete rows")
		return err
	}
	rows, _ := res.RowsAffected()
	logger.Info().Msgf("Successfully deleted %d rows", rows)
	return nil
}
