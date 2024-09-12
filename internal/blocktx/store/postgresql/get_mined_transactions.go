package postgresql

import (
	"context"

	"github.com/lib/pq"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/bitcoin-sv/arc/internal/tracing"
)

func (p *PostgreSQL) GetMinedTransactions(ctx context.Context, hashes [][]byte, blockStatus blocktx_api.Status) ([]store.GetMinedTransactionResult, error) {
	ctx, span := tracing.StartTracing(ctx, "GetMinedTransactions", p.tracingEnabled, p.tracingAttributes...)
	defer tracing.EndTracing(span)

	result := make([]store.GetMinedTransactionResult, 0, len(hashes))

	q := `
		SELECT
			t.hash,
			b.hash,
	  	b.height,
	  	m.merkle_path
	  FROM blocktx.transactions AS t
	  	JOIN blocktx.block_transactions_map AS m ON t.id = m.txid
	  	JOIN blocktx.blocks AS b ON m.blockid = b.id
	  WHERE t.hash = ANY($1) AND b.status = $2
	`

	rows, err := p.db.QueryContext(ctx, q, pq.Array(hashes), blockStatus)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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
