package sql

import (
	"context"
	"fmt"

	"github.com/TAAL-GmbH/arc/blocktx/blocktx_api"
	"github.com/lib/pq"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/gocore"
	"modernc.org/sqlite"
)

// RegisterTransaction registers a transaction in the database
func (s *SQL) RegisterTransaction(ctx context.Context, transaction *blocktx_api.TransactionAndSource) (string, []byte, uint64, error) {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("RegisterTransaction").AddTime(start)
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if transaction.Hash == nil {
		return "", nil, 0, fmt.Errorf("invalid request - no hash")
	}

	if transaction.Source == "" {
		return "", nil, 0, fmt.Errorf("source missing for transaction %s", utils.HexEncodeAndReverseBytes(transaction.Hash))
	}

	q := `INSERT INTO transactions (hash, source) VALUES ($1, $2)`

	if _, err := s.db.ExecContext(ctx, q, transaction.Hash, transaction.Source); err != nil {
		var uniqueConstraint bool

		// Check if this ia a postgres unique constraint violation
		pErr, ok := err.(*pq.Error)
		if ok && pErr.Code == "23505" {
			uniqueConstraint = true
		} else {
			// Check if this ia a sqlite unique constraint violation
			sErr, ok := err.(*sqlite.Error)
			if ok && sErr.Code() == 2067 {
				uniqueConstraint = true
			}
		}

		if !uniqueConstraint {
			return "", nil, 0, err
		}

		// If we reach here, we have a unique violation, which means that the transaction already exists
		q = `UPDATE transactions SET source = $1 WHERE source IS NULL AND hash = $2`
		result, err := s.db.ExecContext(ctx, q, transaction.Source, transaction.Hash)
		if err != nil {
			return "", nil, 0, err
		}

		rows, err := result.RowsAffected()
		if err != nil {
			return "", nil, 0, err
		}

		if rows == 1 {
			// We successfully updated the source and which means it had already been mined
			// so we return the block hash and height
			var blockHash []byte
			var blockHeight uint64

			if err := s.db.QueryRowContext(ctx, `

				SELECT
				 b.hash
				,b.height
				FROM blocks b
				INNER JOIN block_transactions_map m ON m.blockid = b.id
				INNER JOIN transactions t ON m.txid = t.id
				WHERE t.hash = $1
				AND b.orphanedyn = false
			`, transaction.Hash).Scan(&blockHash, &blockHeight); err != nil {
				return "", nil, 0, err
			}

			return transaction.Source, blockHash, blockHeight, nil

		}

		var source string
		if err := s.db.QueryRowContext(ctx, "SELECT source FROM transactions WHERE hash = $1", transaction.Hash).Scan(&source); err != nil {
			return "", nil, 0, err
		}
		return source, nil, 0, nil

	}

	return transaction.Source, nil, 0, nil
}
