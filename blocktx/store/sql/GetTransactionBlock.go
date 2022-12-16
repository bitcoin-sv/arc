package sql

import (
	pb "github.com/TAAL-GmbH/arc/blocktx/api"

	"context"
)

func (s *SQL) GetTransactionBlock(ctx context.Context, transaction *pb.Transaction) (*pb.Block, error) {
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

	var block *pb.Block

	if err := s.db.QueryRowContext(ctx, q, transaction.Hash).Scan(&block); err != nil {
		return nil, err

	}

	return block, nil
}
