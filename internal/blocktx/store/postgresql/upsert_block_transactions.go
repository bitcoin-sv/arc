package postgresql

import (
	"context"
	"errors"
	"fmt"

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

func (p *PostgreSQL) UpsertBlockTransactionsCOPY(ctx context.Context, blockID uint64, txsWithMerklePaths []store.TxWithMerklePath, n int) (err error) {
	ctx, span := tracing.StartTracing(ctx, "UpsertBlockTransactionsCOPY", p.tracingEnabled, append(p.tracingAttributes, attribute.Int("updates", len(txsWithMerklePaths)))...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	dbTx, err := p.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = dbTx.Rollback()
	}()

	// create tmp table
	tmpName := fmt.Sprintf("blocktxtmp_%d_%d", blockID, n)
	tmpTableQ := fmt.Sprintf("CREATE TEMPORARY TABLE %s (blockid BIGINT,txhash BYTEA,mp TEXT);", tmpName)
	_, err = dbTx.ExecContext(ctx, tmpTableQ)
	if err != nil {
		return err
	}

	// insert data to tmp table
	copyStmt, err := dbTx.Prepare(pq.CopyIn(tmpName, "blockid", "txhash", "mp"))
	if err != nil {
		return err
	}

	for _, r := range txsWithMerklePaths {
		_, err = copyStmt.ExecContext(ctx, blockID, r.Hash, r.MerklePath)
		if err != nil {
			return err
		}
	}
	_, err = copyStmt.ExecContext(ctx)
	if err != nil {
		return err
	}

	err = copyStmt.Close()
	if err != nil {
		return err
	}

	// isert transactions
	txUpsertQ := fmt.Sprintf(`WITH inserted_tx AS (
						INSERT INTO blocktx.transactions (hash)
						SELECT txhash
						FROM %s
						ON CONFLICT (hash)
						DO UPDATE SET hash = EXCLUDED.hash
						RETURNING id, hash
					)
					INSERT INTO blocktx.block_transactions_map (
						blockid,
						txid,
						merkle_path
					)
					SELECT tmp.blockid, inserted_tx.id AS txID, tmp.mp AS merkle_path
					FROM inserted_tx
					JOIN %s tmp ON tmp.txhash = inserted_tx.hash
					ON CONFLICT DO NOTHING;
					`, tmpName, tmpName)

	_, err = dbTx.ExecContext(ctx, txUpsertQ)
	if err != nil {
		return err
	}

	_, err = dbTx.ExecContext(ctx, fmt.Sprintf("DROP TABLE %s", tmpName))
	if err != nil {
		return err
	}

	err = dbTx.Commit()
	if err != nil {
		return err
	}

	return nil
}
