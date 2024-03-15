package postgresql

import (
	"context"

	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/ordishs/gocore"
)

func (p *PostgreSQL) TransactionExists(ctx context.Context, hash *chainhash.Hash) (bool, error) {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("TransactionExists").AddTime(start)
	}()

	// Execute the query
	var exists bool
	err := p.db.QueryRow("SELECT EXISTS(SELECT 1 FROM transactions WHERE hash = $1)", hash[:]).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}
