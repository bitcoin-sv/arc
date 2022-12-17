package sql

import (
	"github.com/TAAL-GmbH/arc/blocktx/blocktx_api"

	"context"
)

func (s *SQL) GetTransactionBlocks(ctx context.Context, transaction *blocktx_api.Transaction) (*blocktx_api.Blocks, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	q := `
		SELECT 
		 b.hash
		FROM blocks b
		INNER JOIN block_transactions_map m ON m.blockid = b.id
		INNER JOIN transactions t ON m.txid = t.id
		WHERE t.hash = $1
	`

	rows, err := s.db.QueryContext(ctx, q, transaction.Hash)
	if err != nil {
		return nil, err

	}

	defer rows.Close()

	var blockHash []byte

	var blocks []*blocktx_api.Block

	for rows.Next() {
		err := rows.Scan(&blockHash)
		if err != nil {
			return nil, err
		}

		blocks = append(blocks, &blocktx_api.Block{
			Hash: blockHash,
		})
	}

	return &blocktx_api.Blocks{
		Blocks: blocks,
	}, nil
}
