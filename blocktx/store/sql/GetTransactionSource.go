package sql

import (
	"context"
	"database/sql"

	"github.com/ordishs/gocore"
)

// GetTransactionSource returns the source of a transaction
func (s *SQL) GetTransactionSource(ctx context.Context, txhash []byte) (string, error) {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("GetTransactionSource").AddTime(start)
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	q := `
		SELECT
		 t.source
		FROM transactions t
		WHERE t.hash = $1
	`

	var source sql.NullString

	if err := s.db.QueryRowContext(ctx, q, txhash).Scan(&source); err != nil {
		return "", err
	}

	return source.String, nil
}
