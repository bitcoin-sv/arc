package postgresql

import (
	"context"
	"errors"
	"fmt"

	"github.com/bitcoin-sv/arc/internal/blocktx/store"
)

func (p *PostgreSQL) GetRegisteredTransactions(ctx context.Context, blockId uint64) (registeredTxs []store.TxWithMerklePath, err error) {
	qRegisteredTransactions := `
		SELECT
			t.hash,
			m.merkle_path
		FROM blocktx.transactions t
		JOIN blocktx.block_transactions_map AS m ON t.id = m.txid
		WHERE m.blockid = $1 AND t.is_registered = TRUE
	`

	rows, err := p.db.QueryContext(ctx, qRegisteredTransactions, blockId)
	if err != nil {
		return nil, fmt.Errorf("failed to get registered transactions for block with id %d: %v", blockId, err)
	}
	defer rows.Close()

	registeredRows := make([]store.TxWithMerklePath, 0)

	for rows.Next() {
		var txHash []byte
		var merklePath string
		err = rows.Scan(&txHash, &merklePath)
		if err != nil {
			return nil, errors.Join(store.ErrFailedToGetRows, err)
		}

		registeredRows = append(registeredRows, store.TxWithMerklePath{
			Hash:       txHash,
			MerklePath: merklePath,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error getting registered transactions for block with id %d: %v", blockId, err)
	}

	return registeredRows, nil
}
