package postgresql

import (
	"context"

	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

func (p *PostgreSQL) TransactionExists(ctx context.Context, hash *chainhash.Hash) (bool, error) {

	// Execute the query
	var exists bool
	err := p.db.QueryRow("SELECT EXISTS(SELECT 1 FROM transactions WHERE hash = $1)", hash[:]).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}
