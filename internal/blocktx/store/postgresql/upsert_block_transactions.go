package postgresql

import (
	"context"
	"errors"
	"fmt"

	"github.com/lib/pq"

	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/bitcoin-sv/arc/internal/tracing"
)

// UpsertBlockTransactions upserts the transaction hashes for a given block hash and returns updated registered transactions hashes.
func (p *PostgreSQL) UpsertBlockTransactions(ctx context.Context, blockID uint64, txsWithMerklePaths []store.TxWithMerklePath) (registeredRows []store.TxWithMerklePath, err error) {
	ctx, span := tracing.StartTracing(ctx, "UpdateBlockTransactions", p.tracingEnabled, p.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	txHashesBytes := make([][]byte, len(txsWithMerklePaths))
	merklePaths := make([]string, len(txsWithMerklePaths))
	for i, tx := range txsWithMerklePaths {
		txHashesBytes[i] = tx.Hash
		merklePaths[i] = tx.MerklePath
	}

	qUpsertTransactions := `
		WITH inserted_transactions AS (
				INSERT INTO blocktx.transactions (hash)
				SELECT UNNEST($2::BYTEA[])
				ON CONFLICT (hash)
				DO NOTHING SET hash = EXCLUDED.hash
				RETURNING id, hash
		)

		INSERT INTO blocktx.block_transactions_map (blockid, txid, merkle_path)
		SELECT
				$1::BIGINT,
				it.id,
				t.merkle_path
		FROM inserted_transactions it
		JOIN LATERAL UNNEST($2::BYTEA[], $3::TEXT[]) AS t(hash, merkle_path) ON it.hash = t.hash
		ON CONFLICT(blockid, txid) DO NOTHING;
	`

	qRegisteredTransactions := `
		SELECT
			t.hash,
			m.merkle_path
		FROM blocktx.transactions t
		JOIN blocktx.block_transactions_map AS m ON t.id = m.txid
		WHERE m.blockid = $1 AND t.is_registered = TRUE AND t.hash = ANY($2)
	`

	_, err = p.db.ExecContext(ctx, qUpsertTransactions, blockID, pq.Array(txHashesBytes), pq.Array(merklePaths))
	if err != nil {
		return nil, errors.Join(store.ErrFailedToExecuteTxUpdateQuery, err)
	}

	rows, err := p.db.QueryContext(ctx, qRegisteredTransactions, blockID, pq.Array(txHashesBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to get registered transactions for block with id %d: %v", blockID, err)
	}
	defer rows.Close()

	registeredRows = make([]store.TxWithMerklePath, 0)

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
		return nil, fmt.Errorf("error getting registered transactions for block with id %d: %v", blockID, err)
	}

	return registeredRows, nil
}
