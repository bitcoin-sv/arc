package sql

import (
	"context"
)

// GetTransactionSource returns the source of a transaction
func (s *SQL) GetTransactionSource(ctx context.Context, txhash []byte) (string, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	q := `
		SELECT 
		 t.source
		FROM transactions t
		WHERE t.hash = $1
	`

	var source string

	if err := s.db.QueryRowContext(ctx, q, txhash).Scan(&source); err != nil {
		return "", err
	}

	return source, nil
}
