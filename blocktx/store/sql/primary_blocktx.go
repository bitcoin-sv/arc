package sql

import (
	"context"
	"database/sql"

	"github.com/ordishs/gocore"
	"github.com/pkg/errors"
)

// GetBlockTransactions returns the transaction hashes for a given block hash
func (s *SQL) PrimaryBlocktx(ctx context.Context) (string, error) {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("AmIPrimary").AddTime(start)
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, `SELECT host_name FROM primary_blocktx LIMIT 1`)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}

	defer rows.Close()

	var hostName string

	for rows.Next() {
		err = rows.Scan(&hostName)
		if err != nil {
			return "", err
		}

		return hostName, nil
	}

	return "", nil
}
