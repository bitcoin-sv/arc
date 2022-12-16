package sql

import (
	"context"

	pb "github.com/TAAL-GmbH/arc/blocktx/api"
)

// GetBlockTransactions returns the transaction hashes for a given block hash
func (s *SQL) GetBlockTransactions(ctx context.Context, block *pb.Block) (*pb.Transactions, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	q := `
		SELECT 
		 t.hash
		FROM transactions t
		INNER JOIN block_transactions_map m ON m.txid = t.id
		INNER JOIN blocks b ON m.blockid = b.id
		WHERE b.hash = $1
	`

	rows, err := s.db.QueryContext(ctx, q, block.Hash)
	if err != nil {
		return nil, err

	}

	defer rows.Close()

	var txHash []byte
	var transactions []*pb.Transaction

	for rows.Next() {
		err = rows.Scan(&txHash)
		if err != nil {
			return nil, err
		}

		transactions = append(transactions, &pb.Transaction{
			Hash: txHash,
		})
	}

	return &pb.Transactions{
		Transactions: transactions,
	}, nil
}
