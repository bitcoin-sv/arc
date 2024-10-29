package postgresql

import (
	"context"

	"github.com/lib/pq"
	"github.com/libsv/go-p2p/chaincfg/chainhash"

	"github.com/bitcoin-sv/arc/internal/blocktx/store"
)

func (p *PostgreSQL) GetMinedTransactions(ctx context.Context, hashes []*chainhash.Hash) ([]store.GetMinedTransactionResult, error) {
	ctx, span := p.startTracing(ctx, "GetMinedTransactions")
	defer p.endTracing(span)

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
	  	m.merkle_path
	  FROM blocktx.transactions AS t
	  	JOIN blocktx.block_transactions_map AS m ON t.id = m.txid
	  	JOIN blocktx.blocks AS b ON m.blockid = b.id
	  WHERE t.hash = ANY($1)
	`

	rows, err := p.db.QueryContext(ctx, q, pq.Array(hashSlice))
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
