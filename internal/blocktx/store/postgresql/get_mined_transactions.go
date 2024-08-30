package postgresql

import (
	"context"

	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/lib/pq"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"go.opentelemetry.io/otel/trace"
)

func (p *PostgreSQL) GetMinedTransactions(ctx context.Context, hashes []*chainhash.Hash) ([]store.GetMinedTransactionResult, error) {
	if tracer != nil {
		var span trace.Span
		ctx, span = tracer.Start(ctx, "GetMinedTransactions")
		defer span.End()
	}

	var hashSlice [][]byte
	for _, hash := range hashes {
		hashSlice = append(hashSlice, hash[:])
	}

	result := make([]store.GetMinedTransactionResult, 0, len(hashSlice))

	q := `
		SELECT
			t.hash,
			b.hash,
	  	b.height,
	  	t.merkle_path
	  FROM blocktx.transactions as t
	  	JOIN blocktx.block_transactions_map ON t.id = blocktx.block_transactions_map.txid
	  	JOIN blocktx.blocks as b ON blocktx.block_transactions_map.blockid = b.id
	  WHERE t.hash = ANY($1)
	`

	rows, err := p.db.QueryContext(ctx, q, pq.Array(hashSlice))
	if err != nil {
		return nil, err
	}

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
