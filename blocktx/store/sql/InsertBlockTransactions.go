package sql

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/TAAL-GmbH/arc/blocktx/blocktx_api"
)

// InsertBlockTransactions inserts the transaction hashes for a given block hash
func (s *SQL) InsertBlockTransactions(ctx context.Context, blockId uint64, transactions []*blocktx_api.TransactionAndSource) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	qTx := `
			INSERT INTO transactions (hash) VALUES ($1)
			ON CONFLICT DO NOTHING
			RETURNING id
			;
		`
	qMap := `
		INSERT INTO block_transactions_map (
		 blockid
		,txid
		,pos
		) VALUES
	`

	qMapRows := make([]string, 0, len(transactions))
	for pos, tx := range transactions {
		var txid uint64

		if err := s.db.QueryRowContext(ctx, qTx, tx.Hash).Scan(&txid); err != nil {
			if err == sql.ErrNoRows {
				if err = s.db.QueryRowContext(ctx, `
					SELECT id 
					FROM transactions 
					WHERE hash = $1
				`, tx.Hash).Scan(&txid); err != nil {
					return err
				}
			} else {
				return err
			}
		}

		// this is ugly, but a lot faster than sprintf
		qMapRows = append(qMapRows, fmt.Sprintf(" ("+strconv.FormatUint(blockId, 10)+", "+strconv.FormatUint(txid, 10)+", "+strconv.Itoa(pos)+")"))

		// maximum of 1000 rows per query is allowed in postgres
		if len(qMapRows) >= 1000 {
			if err := s.bulkInsert(ctx, qMap, qMapRows); err != nil {
				return err
			}
			qMapRows = qMapRows[:0]
		}
	}

	// insert the remaining rows
	if len(qMapRows) > 0 {
		if err := s.bulkInsert(ctx, qMap, qMapRows); err != nil {
			return err
		}
	}

	return nil
}

func (s *SQL) bulkInsert(ctx context.Context, queryTemplate string, queryRows []string) error {
	// remove the last comma
	query := queryTemplate + strings.Join(queryRows, ",")
	query += ` ON CONFLICT DO NOTHING;`

	// insert the block / transaction map in 1 query
	_, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return err
	}

	return nil
}
