package sql

import (
	"context"
	"database/sql"
	"errors"

	"github.com/ordishs/gocore"
)

// GetBlockTransactions returns the transaction hashes for a given block hash.
func (s *SQL) PrimaryBlocktx(ctx context.Context) (string, error) {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("AmIPrimary").AddTime(start)
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var hostName string
	err := s.db.QueryRowContext(ctx, `SELECT host_name FROM primary_blocktx LIMIT 1`).Scan(&hostName)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}

	return hostName, nil
}
