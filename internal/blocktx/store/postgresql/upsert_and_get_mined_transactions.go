package postgresql

import (
	"context"
	"errors"

	"github.com/bitcoin-sv/arc/internal/blocktx/store"

	"github.com/bitcoin-sv/arc/internal/tracing"
	"github.com/lib/pq"
)

func (p *PostgreSQL) UpsertAndGetMinedTransactions(ctx context.Context, txHashes [][]byte) (result []store.GetMinedTransactionResult, err error) {
	ctx, span := tracing.StartTracing(ctx, "UpsertAndGetMinedTransactions", p.tracingEnabled, p.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	const q = `
		WITH inserted_transactions AS (
			INSERT INTO blocktx.transactions (hash, is_registered)
				SELECT hash, TRUE
				FROM UNNEST ($1::BYTEA[]) as hash
			ON CONFLICT (hash) DO UPDATE
				SET is_registered = TRUE
			RETURNING hash
		)

		SELECT
			t.hash,
			b.hash,
	  	b.height,
	  	m.merkle_path
	  FROM blocktx.transactions AS t
	  	JOIN blocktx.block_transactions_map AS m ON t.id = m.txid
	  	JOIN blocktx.blocks AS b ON m.blockid = b.id
	  WHERE t.hash = ANY(SELECT hash FROM inserted_transactions)
	`

	rows, err := p.db.QueryContext(ctx, q, pq.Array(txHashes))
	if err != nil {
		return nil, errors.Join(store.ErrFailedToUpsertTransactions, err)
	}
	defer rows.Close()

	result = make([]store.GetMinedTransactionResult, 0, len(txHashes))
	for rows.Next() {
		var txHash []byte
		var blockHash []byte
		var blockHeight uint64
		var merklePath string

		err = rows.Scan(
			&txHash,
			&blockHash,
			&blockHeight,
			&merklePath,
		)
		if err != nil {
			return nil, err
		}

		result = append(result, store.GetMinedTransactionResult{
			TxHash:      txHash,
			BlockHash:   blockHash,
			BlockHeight: blockHeight,
			MerklePath:  merklePath,
		})
	}

	return result, nil
}
