package sql

import (
	pb "github.com/TAAL-GmbH/arc/blocktx_api"

	"context"
)

func (s *SQL) GetTransactionBlocks(ctx context.Context, transaction *pb.Transaction) (*pb.Blocks, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	q := `
		SELECT 
		 b.hash
		FROM block_transactions t
		INNER JOIN blocks b ON b.id = t.blockid
		WHERE t.txhash = $1
	`

	rows, err := s.db.QueryContext(ctx, q, transaction.Hash)
	if err != nil {
		return nil, err

	}

	defer rows.Close()

	var blockHash []byte

	var blocks []*pb.Block

	for rows.Next() {
		err := rows.Scan(&blockHash)
		if err != nil {
			return nil, err
		}

		blocks = append(blocks, &pb.Block{
			Hash: blockHash,
		})
	}

	return &pb.Blocks{
		Blocks: blocks,
	}, nil
}
