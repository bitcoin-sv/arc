package sql

import (
	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/ordishs/gocore"

	"context"
)

const (
	queryGetBlockHashHeightForTransactionHash = `
		SELECT
		 b.hash, b.height
		FROM blocks b
		INNER JOIN block_transactions_map m ON m.blockid = b.id
		INNER JOIN transactions t ON m.txid = t.id
		WHERE t.hash = $1
		AND b.orphanedyn = false
	`
)

func (s *SQL) GetTransactionBlock(ctx context.Context, transaction *blocktx_api.Transaction) (*blocktx_api.Block, error) {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("GetTransactionBlock").AddTime(start)
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	block := &blocktx_api.Block{}
	if err := s.db.QueryRowContext(ctx, queryGetBlockHashHeightForTransactionHash, transaction.Hash).Scan(&block.Hash, &block.Height); err != nil {
		return nil, err

	}

	return block, nil
}
