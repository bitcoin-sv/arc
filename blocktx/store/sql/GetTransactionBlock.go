package sql

import (
	pb "github.com/TAAL-GmbH/arcblocktx_api"

	"context"
)

func (s *SQL) GetTransactionBlock(ctx context.Context, transaction *pb.Transaction) (*pb.Block, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	q := `
		SELECT 
		 b.hash
		FROM block_transactions t
		INNER JOIN blocks b ON b.id = t.blockid
		WHERE t.txhash = $1
		AND b.orphanedyn = false
	`

	var block *pb.Block

	if err := s.db.QueryRowContext(ctx, q, transaction.Hash).Scan(&block); err != nil {
		return nil, err

	}

	return block, nil
}
