package sql

import (
	pb "github.com/TAAL-GmbH/arc/blocktx_api"

	"context"
)

// InsertBlockTransactions inserts the transaction hashes for a given block hash
func (s *SQL) InsertBlockTransactions(ctx context.Context, blockId uint64, transactions []*pb.Transaction) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	q := `
		INSERT INTO block_transactions_map (
		 blockid
		,txid
		) VALUES (
		 $1
		,(SELECT id FROM transactions WHERE hash = $2)
		)
		ON CONFLICT DO NOTHING
	`

	txn, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	for _, tx := range transactions {
		_, err = txn.ExecContext(ctx, q, blockId, tx.Hash)
		if err != nil {
			return err
		}
	}

	if err := txn.Commit(); err != nil {
		return err
	}

	return nil
}
