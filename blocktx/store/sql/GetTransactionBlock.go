package sql

import (
	"github.com/TAAL-GmbH/arc/blocktx/blocktx_api"
	"github.com/ordishs/gocore"

	"context"
)

func (s *SQL) GetTransactionBlock(ctx context.Context, transaction *blocktx_api.Transaction) (*blocktx_api.Block, error) {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("GetTransactionBlock").AddTime(start)
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	q := `
		SELECT
		 b.hash
		FROM blocks b
		INNER JOIN block_transactions_map m ON m.blockid = b.id
		INNER JOIN transactions t ON m.txid = t.id
		WHERE t.hash = $1
		AND b.orphanedyn = false
	`

	var block *blocktx_api.Block

	if err := s.db.QueryRowContext(ctx, q, transaction.Hash).Scan(&block); err != nil {
		return nil, err

	}

	return block, nil
}
