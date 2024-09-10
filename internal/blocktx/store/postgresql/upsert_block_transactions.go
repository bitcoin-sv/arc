package postgresql

import (
	"context"
	"fmt"

	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/lib/pq"
	"go.opentelemetry.io/otel/trace"
)

// UpsertBlockTransactions upserts the transaction hashes for a given block hash and returns updated registered transactions hashes.
func (p *PostgreSQL) UpsertBlockTransactions(ctx context.Context, blockId uint64, txsWithMerklePaths []store.TxWithMerklePath) ([]store.TxWithMerklePath, error) {
	if tracer != nil {
		var span trace.Span
		ctx, span = tracer.Start(ctx, "UpdateBlockTransactions")
		defer span.End()
	}

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
				DO UPDATE SET hash = transactions.hash
				RETURNING id, hash
		)

		INSERT INTO blocktx.block_transactions_map (blockid, txid, merkle_path)
		SELECT
				$1::BIGINT,
				it.id,
				t.merkle_path
		FROM inserted_transactions it
		JOIN LATERAL UNNEST($2::BYTEA[], $3::TEXT[]) AS t(hash, merkle_path) ON it.hash = t.hash;
	`

	qRegisteredTransactions := `
		SELECT
			t.hash,
			m.merkle_path
		FROM blocktx.transactions t
	  JOIN blocktx.block_transactions_map AS m ON t.id = m.txid
		WHERE m.blockid = $1 AND t.is_registered = TRUE
	`

	_, err := p.db.ExecContext(ctx, qUpsertTransactions, blockId, pq.Array(txHashesBytes), pq.Array(merklePaths))
	if err != nil {
		return nil, fmt.Errorf("failed to execute transactions upsert query: %v", err)
	}

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
			return nil, fmt.Errorf("failed to get rows: %v", err)
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
