package postgresql

import (
	"context"
	"errors"

	"github.com/lib/pq"
	"go.opentelemetry.io/otel/attribute"

	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/bitcoin-sv/arc/internal/tracing"
)

// UpsertBlockTransactions upserts the transaction hashes for a given block hash and returns updated registered transactions hashes.
func (p *PostgreSQL) UpsertBlockTransactions(ctx context.Context, blockID uint64, txsWithMerklePaths []store.TxWithMerklePath) (err error) {
	ctx, span := tracing.StartTracing(ctx, "UpsertBlockTransactions", p.tracingEnabled, append(p.tracingAttributes, attribute.Int("updates", len(txsWithMerklePaths)))...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	txHashes := make([][]byte, len(txsWithMerklePaths))
	blockIDs := make([]uint64, len(txsWithMerklePaths))
	merklePaths := make([]string, len(txsWithMerklePaths))
	for pos, tx := range txsWithMerklePaths {
		txHashes[pos] = tx.Hash
		merklePaths[pos] = tx.MerklePath
		blockIDs[pos] = blockID
	}

	qBulkUpsert := `
		INSERT INTO blocktx.transactions (hash)
			SELECT UNNEST($1::BYTEA[])
			ON CONFLICT (hash)
			DO UPDATE SET hash = EXCLUDED.hash
		RETURNING id`

	rows, err := p.db.QueryContext(ctx, qBulkUpsert, pq.Array(txHashes))
	if err != nil {
		return errors.Join(store.ErrFailedToUpsertTransactions, err)
	}

	counter := 0
	txIDs := make([]uint64, len(txsWithMerklePaths))
	for rows.Next() {
		var txID uint64
		err = rows.Scan(&txID)
		if err != nil {
			return errors.Join(store.ErrFailedToGetRows, err)
		}

		txIDs[counter] = txID
		counter++
	}

	if len(txIDs) != len(txsWithMerklePaths) {
		return errors.Join(store.ErrMismatchedTxIDsAndMerklePathLength, err)
	}

	const qMapInsert = `
		INSERT INTO blocktx.block_transactions_map (
			 blockid
			,txid
			,merkle_path
			)
		SELECT * FROM UNNEST($1::INT[], $2::INT[], $3::TEXT[])
		ON CONFLICT DO NOTHING
		`
	_, err = p.db.ExecContext(ctx, qMapInsert, pq.Array(blockIDs), pq.Array(txIDs), pq.Array(merklePaths))
	if err != nil {
		return errors.Join(store.ErrFailedToUpsertBlockTransactionsMap, err)
	}

	return nil
}
