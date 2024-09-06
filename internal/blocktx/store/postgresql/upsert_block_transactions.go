package postgresql

import (
	"context"
	"fmt"

	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/lib/pq"
	"go.opentelemetry.io/otel/trace"
)

// UpsertBlockTransactions upserts the transaction hashes for a given block hash and returns updated registered transactions hashes.
func (p *PostgreSQL) UpsertBlockTransactions(ctx context.Context, blockId uint64, txsWithMerklePaths []store.UpsertBlockTransactionsResult) ([]store.UpsertBlockTransactionsResult, error) {
	if tracer != nil {
		var span trace.Span
		ctx, span = tracer.Start(ctx, "UpdateBlockTransactions")
		defer span.End()
	}

	blockIDs := make([]uint64, len(txsWithMerklePaths))
	txHashesBytes := make([][]byte, len(txsWithMerklePaths))
	merklePaths := make([]string, len(txsWithMerklePaths))
	for i, tx := range txsWithMerklePaths {
		blockIDs[i] = blockId
		txHashesBytes[i] = tx.TxHash
		merklePaths[i] = tx.MerklePath
	}

	qUpsertTransactions := `
		INSERT INTO blocktx.transactions (hash)
		SELECT UNNEST($1::BYTEA[])
		ON CONFLICT DO NOTHING
	`

	// TODO: start transaction

	_, err := p.db.ExecContext(ctx, qUpsertTransactions, pq.Array(txHashesBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to execute transaction update query: %v", err)
	}

	qUpsertBlockTxsMap := `
		INSERT INTO blocktx.block_transactions_map (
			blockid
			,tx_hash
			,merkle_path
		)
		SELECT * FROM UNNEST($1::INT[], $2::INT[], $3::TEXT[])
		ON CONFLICT DO NOTHING
	`
	_, err = p.db.ExecContext(ctx, qUpsertBlockTxsMap, pq.Array(blockIDs), pq.Array(txHashesBytes), pq.Array(merklePaths))
	if err != nil {
		return nil, fmt.Errorf("failed to bulk insert transactions into block transactions map for block with id %d: %v", blockId, err)
	}

	qRegisteredTransactions := `
		SELECT
			t.hash,
			m.merkle_path
		FROM blocktx.transactions AS t
	  JOIN blocktx.block_transactions_map AS m ON t.id = m.txid
		WHERE m.blockid = $1 AND t.is_registered = TRUE
	`
	rows, err := p.db.QueryContext(ctx, qRegisteredTransactions, blockId)
	if err != nil {
		return nil, fmt.Errorf("failed to get registered transactions for block with id %d: %v", blockId, err)
	}

	registeredRows := make([]store.UpsertBlockTransactionsResult, 0)

	for rows.Next() {
		var txHash []byte
		var merklePath string
		err = rows.Scan(&txHash, &merklePath)
		if err != nil {
			return nil, fmt.Errorf("failed to get rows: %v", err)
		}

		registeredRows = append(registeredRows, store.UpsertBlockTransactionsResult{
			TxHash:     txHash,
			MerklePath: merklePath,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error getting registered transactions for block with id %d: %v", blockId, err)
	}

	return registeredRows, nil
}

func (p *PostgreSQL) insertTxsIntoBlockMap(ctx context.Context, blockId uint64, blockIDs, txIDs []uint64) error {
	qMap := `
		INSERT INTO blocktx.block_transactions_map (
			blockid
			,txid
		)
		SELECT * FROM UNNEST($1::INT[], $2::INT[])
		ON CONFLICT DO NOTHING
		`
	_, err := p.db.ExecContext(ctx, qMap, pq.Array(blockIDs), pq.Array(txIDs))
	if err != nil {
		return fmt.Errorf("failed to bulk insert transactions into block transactions map for block with id %d: %v", blockId, err)
	}

	return nil
}
